package dlna

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/dms/soap"
	"github.com/anacrolix/dms/ssdp"
	"github.com/anacrolix/dms/upnp"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/dlna/dlnaflags"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

func init() {
	dlnaflags.AddFlags(Command.Flags())
	vfsflags.AddFlags(Command.Flags())
}

// Command definition for cobra.
var Command = &cobra.Command{
	Use:   "dlna remote:path",
	Short: `Serve remote:path over DLNA`,
	Long: `rclone serve dlna is a DLNA media server for media stored in a rclone remote. Many
devices, such as the Xbox and PlayStation, can automatically discover this server in the LAN
and play audio/video from it. VLC is also supported. Service discovery uses UDP multicast
packets (SSDP) and will thus only work on LANs.

Rclone will list all files present in the remote, without filtering based on media formats or
file extensions. Additionally, there is no media transcoding support. This means that some
players might show files that they are not able to play back correctly.

` + dlnaflags.Help + vfs.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		cmd.Run(false, false, command, func() error {
			s := newServer(f, &dlnaflags.Opt)
			if err := s.Serve(); err != nil {
				log.Fatal(err)
			}
			s.Wait()
			return nil
		})
	},
}

const (
	serverField         = "Linux/3.4 DLNADOC/1.50 UPnP/1.0 DMS/1.0"
	rootDeviceType      = "urn:schemas-upnp-org:device:MediaServer:1"
	rootDeviceModelName = "rclone"
	resPath             = "/res"
	rootDescPath        = "/rootDesc.xml"
	serviceControlURL   = "/ctl"
)

// Groups the service definition with its XML description.
type service struct {
	upnp.Service
	SCPD string
}

// Exposed UPnP AV services.
var services = []*service{
	{
		Service: upnp.Service{
			ServiceType: "urn:schemas-upnp-org:service:ContentDirectory:1",
			ServiceId:   "urn:upnp-org:serviceId:ContentDirectory",
			ControlURL:  serviceControlURL,
		},
		SCPD: contentDirectoryServiceDescription,
	},
}

func devices() []string {
	return []string{
		"urn:schemas-upnp-org:device:MediaServer:1",
	}
}

func serviceTypes() (ret []string) {
	for _, s := range services {
		ret = append(ret, s.ServiceType)
	}
	return
}

type server struct {
	// The service SOAP handler keyed by service URN.
	services map[string]UPnPService

	Interfaces []net.Interface

	HTTPConn       net.Listener
	httpListenAddr string
	httpServeMux   *http.ServeMux

	rootDeviceUUID string
	rootDescXML    []byte

	FriendlyName string

	// For waiting on the listener to close
	waitChan chan struct{}

	// Time interval between SSPD announces
	AnnounceInterval time.Duration

	f   fs.Fs
	vfs *vfs.VFS
}

func newServer(f fs.Fs, opt *dlnaflags.Options) *server {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = ""
	} else {
		hostName = " (" + hostName + ")"
	}

	s := &server{
		AnnounceInterval: 10 * time.Second,
		FriendlyName:     "rclone" + hostName,

		httpListenAddr: opt.ListenAddr,

		f:   f,
		vfs: vfs.New(f, &vfsflags.Opt),
	}

	s.initServicesMap()
	s.listInterfaces()

	s.httpServeMux = http.NewServeMux()
	s.rootDeviceUUID = makeDeviceUUID(s.FriendlyName)
	s.rootDescXML, err = xml.MarshalIndent(
		upnp.DeviceDesc{
			SpecVersion: upnp.SpecVersion{Major: 1, Minor: 0},
			Device: upnp.Device{
				DeviceType:   rootDeviceType,
				FriendlyName: s.FriendlyName,
				Manufacturer: "rclone (rclone.org)",
				ModelName:    rootDeviceModelName,
				UDN:          s.rootDeviceUUID,
				ServiceList: func() (ss []upnp.Service) {
					for _, s := range services {
						ss = append(ss, s.Service)
					}
					return
				}(),
			},
		},
		" ", "  ")
	if err != nil {
		// Contents are hardcoded, so this will never happen in production.
		log.Panicf("Marshal root descriptor XML: %v", err)
	}
	s.rootDescXML = append([]byte(`<?xml version="1.0"?>`), s.rootDescXML...)
	s.initMux(s.httpServeMux)

	return s
}

// UPnPService is the interface for the SOAP service.
type UPnPService interface {
	Handle(action string, argsXML []byte, r *http.Request) (respArgs map[string]string, err error)
	Subscribe(callback []*url.URL, timeoutSeconds int) (sid string, actualTimeout int, err error)
	Unsubscribe(sid string) error
}

// initServicesMap is called during initialization of the server to prepare some internal datastructures.
func (s *server) initServicesMap() {
	urn, err := upnp.ParseServiceType(services[0].ServiceType)
	if err != nil {
		// The service type is hardcoded, so this error should never happen.
		log.Panicf("ParseServiceType: %v", err)
	}
	s.services = map[string]UPnPService{
		urn.Type: &contentDirectoryService{
			server: s,
		},
	}
	return
}

// listInterfaces is called during initialization of the server to list the network interfaces
// on the machine.
func (s *server) listInterfaces() {
	ifs, err := net.Interfaces()
	if err != nil {
		fs.Errorf(s.f, "list network interfaces: %v", err)
		return
	}

	var tmp []net.Interface
	for _, intf := range ifs {
		if intf.Flags&net.FlagUp == 0 || intf.MTU <= 0 {
			continue
		}
		s.Interfaces = append(s.Interfaces, intf)
		tmp = append(tmp, intf)
	}
}

func (s *server) initMux(mux *http.ServeMux) {
	mux.HandleFunc(resPath, func(w http.ResponseWriter, r *http.Request) {
		remotePath := r.URL.Query().Get("path")
		node, err := s.vfs.Stat(remotePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Length", strconv.FormatInt(node.Size(), 10))

		file := node.(*vfs.File)
		in, err := file.Open(os.O_RDONLY)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		defer fs.CheckClose(in, &err)

		http.ServeContent(w, r, remotePath, node.ModTime(), in)
		return
	})

	mux.HandleFunc(rootDescPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", `text/xml; charset="utf-8"`)
		w.Header().Set("content-length", fmt.Sprint(len(s.rootDescXML)))
		w.Header().Set("server", serverField)
		_, err := w.Write(s.rootDescXML)
		if err != nil {
			fs.Errorf(s, "Failed to serve root descriptor XML: %v", err)
		}
	})

	// Install handlers to serve SCPD for each UPnP service.
	for _, s := range services {
		p := path.Join("/scpd", s.ServiceId)
		s.SCPDURL = p

		mux.HandleFunc(s.SCPDURL, func(serviceDesc string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", `text/xml; charset="utf-8"`)
				http.ServeContent(w, r, ".xml", time.Time{}, bytes.NewReader([]byte(serviceDesc)))
			}
		}(s.SCPD))
	}

	mux.HandleFunc(serviceControlURL, s.serviceControlHandler)
}

// Handle a service control HTTP request.
func (s *server) serviceControlHandler(w http.ResponseWriter, r *http.Request) {
	soapActionString := r.Header.Get("SOAPACTION")
	soapAction, err := upnp.ParseActionHTTPHeader(soapActionString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var env soap.Envelope
	if err := xml.NewDecoder(r.Body).Decode(&env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.Header().Set("Ext", "")
	w.Header().Set("server", serverField)
	soapRespXML, code := func() ([]byte, int) {
		respArgs, err := s.soapActionResponse(soapAction, env.Body.Action, r)
		if err != nil {
			upnpErr := upnp.ConvertError(err)
			return mustMarshalXML(soap.NewFault("UPnPError", upnpErr)), 500
		}
		return marshalSOAPResponse(soapAction, respArgs), 200
	}()
	bodyStr := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8" standalone="yes"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>%s</s:Body></s:Envelope>`, soapRespXML)
	w.WriteHeader(code)
	if _, err := w.Write([]byte(bodyStr)); err != nil {
		log.Print(err)
	}
}

// Handle a SOAP request and return the response arguments or UPnP error.
func (s *server) soapActionResponse(sa upnp.SoapAction, actionRequestXML []byte, r *http.Request) (map[string]string, error) {
	service, ok := s.services[sa.Type]
	if !ok {
		// TODO: What's the invalid service error?
		return nil, upnp.Errorf(upnp.InvalidActionErrorCode, "Invalid service: %s", sa.Type)
	}
	return service.Handle(sa.Action, actionRequestXML, r)
}

// Serve runs the server - returns the error only if
// the listener was not started; does not block, so
// use s.Wait() to block on the listener indefinitely.
func (s *server) Serve() (err error) {
	if s.HTTPConn == nil {
		s.HTTPConn, err = net.Listen("tcp", s.httpListenAddr)
		if err != nil {
			return
		}
	}

	go func() {
		s.startSSDP()
	}()

	go func() {
		fs.Logf(s.f, "Serving HTTP on %s", s.HTTPConn.Addr().String())

		err = s.serveHTTP()
		if err != nil {
			fs.Logf(s.f, "Error on serving HTTP server: %v", err)
		}
	}()

	return nil
}

// Wait blocks while the listener is open.
func (s *server) Wait() {
	<-s.waitChan
}

func (s *server) Close() {
	err := s.HTTPConn.Close()
	if err != nil {
		fs.Errorf(s.f, "Error closing HTTP server: %v", err)
		return
	}
	close(s.waitChan)
}

// Run SSDP (multicast for server discovery) on all interfaces.
func (s *server) startSSDP() {
	active := 0
	stopped := make(chan struct{})
	for _, intf := range s.Interfaces {
		active++
		go func(intf2 net.Interface) {
			defer func() {
				stopped <- struct{}{}
			}()
			s.ssdpInterface(intf2)
		}(intf)
	}
	for active > 0 {
		<-stopped
		active--
	}
}

// Run SSDP server on an interface.
func (s *server) ssdpInterface(intf net.Interface) {
	// Figure out which HTTP location to advertise based on the interface IP.
	advertiseLocationFn := func(ip net.IP) string {
		url := url.URL{
			Scheme: "http",
			Host: (&net.TCPAddr{
				IP:   ip,
				Port: s.HTTPConn.Addr().(*net.TCPAddr).Port,
			}).String(),
			Path: rootDescPath,
		}
		return url.String()
	}

	ssdpServer := ssdp.Server{
		Interface:      intf,
		Devices:        devices(),
		Services:       serviceTypes(),
		Location:       advertiseLocationFn,
		Server:         serverField,
		UUID:           s.rootDeviceUUID,
		NotifyInterval: s.AnnounceInterval,
	}

	// An interface with these flags should be valid for SSDP.
	const ssdpInterfaceFlags = net.FlagUp | net.FlagMulticast

	if err := ssdpServer.Init(); err != nil {
		if intf.Flags&ssdpInterfaceFlags != ssdpInterfaceFlags {
			// Didn't expect it to work anyway.
			return
		}
		if strings.Contains(err.Error(), "listen") {
			// OSX has a lot of dud interfaces. Failure to create a socket on
			// the interface are what we're expecting if the interface is no
			// good.
			return
		}
		log.Printf("Error creating ssdp server on %s: %s", intf.Name, err)
		return
	}
	defer ssdpServer.Close()
	log.Println("Started SSDP on", intf.Name)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if err := ssdpServer.Serve(); err != nil {
			log.Printf("%q: %q\n", intf.Name, err)
		}
	}()
	select {
	case <-s.waitChan:
		// Returning will close the server.
	case <-stopped:
	}
}

func (s *server) serveHTTP() error {
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.httpServeMux.ServeHTTP(w, r)
		}),
	}
	err := srv.Serve(s.HTTPConn)
	select {
	case <-s.waitChan:
		return nil
	default:
		return err
	}
}
