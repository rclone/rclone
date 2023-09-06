package hdfs

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"sort"
	"strings"

	"github.com/colinmarc/hdfs/v2/hadoopconf"
	hadoop "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_common"
	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"github.com/colinmarc/hdfs/v2/internal/rpc"
	"github.com/colinmarc/hdfs/v2/internal/transfer"
	krb "github.com/jcmturner/gokrb5/v8/client"
)

type dialContext func(ctx context.Context, network, addr string) (net.Conn, error)

const (
	DataTransferProtectionAuthentication = "authentication"
	DataTransferProtectionIntegrity      = "integrity"
	DataTransferProtectionPrivacy        = "privacy"
)

// Client represents a connection to an HDFS cluster. A Client will
// automatically maintain leases for any open files, preventing other clients
// from modifying them, until Close is called.
type Client struct {
	namenode *rpc.NamenodeConnection
	options  ClientOptions

	defaults      *hdfs.FsServerDefaultsProto
	encryptionKey *hdfs.DataEncryptionKeyProto
}

// ClientOptions represents the configurable options for a client.
// The NamenodeDialFunc and DatanodeDialFunc options can be used to set
// connection timeouts:
//
//    dialFunc := (&net.Dialer{
//        Timeout:   30 * time.Second,
//        KeepAlive: 30 * time.Second,
//        DualStack: true,
//    }).DialContext
//
//    options := ClientOptions{
//        Addresses: []string{"nn1:9000"},
//        NamenodeDialFunc: dialFunc,
//        DatanodeDialFunc: dialFunc,
//    }
type ClientOptions struct {
	// Addresses specifies the namenode(s) to connect to.
	Addresses []string
	// User specifies which HDFS user the client will act as. It is required
	// unless kerberos authentication is enabled, in which case it is overridden
	// by the username set in KerberosClient.
	User string
	// UseDatanodeHostname specifies whether the client should connect to the
	// datanodes via hostname (which is useful in multi-homed setups) or IP
	// address, which may be required if DNS isn't available.
	UseDatanodeHostname bool
	// NamenodeDialFunc is used to connect to the namenodes. If nil, then
	// (&net.Dialer{}).DialContext is used.
	NamenodeDialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
	// DatanodeDialFunc is used to connect to the datanodes. If nil, then
	// (&net.Dialer{}).DialContext is used.
	DatanodeDialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
	// KerberosClient is used to connect to kerberized HDFS clusters. If provided,
	// the client will always mutually authenticate when connecting to the
	// namenode(s).
	KerberosClient *krb.Client
	// KerberosServicePrincipleName specifies the Service Principle Name
	// (<SERVICE>/<FQDN>) for the namenode(s). Like in the
	// dfs.namenode.kerberos.principal property of core-site.xml, the special
	// string '_HOST' can be substituted for the address of the namenode in a
	// multi-namenode setup (for example: 'nn/_HOST'). It is required if
	// KerberosClient is provided.
	KerberosServicePrincipleName string
	// DataTransferProtection specifies whether or not authentication, data
	// signature integrity checks, and wire encryption is required when
	// communicating the the datanodes. A value of "authentication" implies
	// just authentication, a value of "integrity" implies both authentication
	// and integrity checks, and a value of "privacy" implies all three. The
	// Client may negotiate a higher level of protection if it is requested
	// by the datanode; for example, if the datanode and namenode hdfs-site.xml
	// has dfs.encrypt.data.transfer enabled, this setting is ignored and
	// a level of "privacy" is used.
	DataTransferProtection string
	// skipSaslForPrivilegedDatanodePorts implements a strange edge case present
	// in the official java client. If data.transfer.protection is set but not
	// dfs.encrypt.data.transfer, and the datanode is running on a privileged
	// port, the client connects without doing a SASL handshake. This field is
	// only set by ClientOptionsFromConf.
	skipSaslForPrivilegedDatanodePorts bool
}

// ClientOptionsFromConf attempts to load any relevant configuration options
// from the given Hadoop configuration and create a ClientOptions struct
// suitable for creating a Client. Currently this sets the following fields
// on the resulting ClientOptions:
//
//   // Determined by fs.defaultFS (or the deprecated fs.default.name), or
//   // fields beginning with dfs.namenode.rpc-address.
//   Addresses []string
//
//   // Determined by dfs.client.use.datanode.hostname.
//   UseDatanodeHostname bool
//
//   // Set to a non-nil but empty client (without credentials) if the value of
//   // hadoop.security.authentication is 'kerberos'. It must then be replaced
//   // with a credentialed Kerberos client.
//   KerberosClient *krb.Client
//
//   // Determined by dfs.namenode.kerberos.principal, with the realm
//   // (everything after the first '@') chopped off.
//   KerberosServicePrincipleName string
//
//   // Determined by dfs.data.transfer.protection or dfs.encrypt.data.transfer
//   // (in the latter case, it is set to 'privacy').
//   DataTransferProtection string
//
// Because of the way Kerberos can be forced by the Hadoop configuration but not
// actually configured, you should check for whether KerberosClient is set in
// the resulting ClientOptions before proceeding:
//
//   options := ClientOptionsFromConf(conf)
//   if options.KerberosClient != nil {
//      // Replace with a valid credentialed client.
//      options.KerberosClient = getKerberosClient()
//   }
func ClientOptionsFromConf(conf hadoopconf.HadoopConf) ClientOptions {
	options := ClientOptions{Addresses: conf.Namenodes()}

	options.UseDatanodeHostname = (conf["dfs.client.use.datanode.hostname"] == "true")

	if strings.ToLower(conf["hadoop.security.authentication"]) == "kerberos" {
		// Set an empty KerberosClient here so that the user is forced to either
		// unset it (disabling kerberos altogether) or replace it with a valid
		// client. If the user does neither, NewClient will return an error.
		options.KerberosClient = &krb.Client{}
	}

	if conf["dfs.namenode.kerberos.principal"] != "" {
		options.KerberosServicePrincipleName = strings.Split(conf["dfs.namenode.kerberos.principal"], "@")[0]
	}

	// Note that we take the highest setting, rather than allowing a range of
	// alternatives. 'authentication', 'integrity', and 'privacy' are
	// alphabetical for our convenience.
	dataTransferProt := strings.Split(
		strings.ToLower(conf["dfs.data.transfer.protection"]), ",")
	sort.Strings(dataTransferProt)

	for _, val := range dataTransferProt {
		switch val {
		case "privacy":
			options.DataTransferProtection = "privacy"
		case "integrity":
			options.DataTransferProtection = "integrity"
		case "authentication":
			options.DataTransferProtection = "authentication"
		}
	}

	if strings.ToLower(conf["dfs.encrypt.data.transfer"]) == "true" {
		options.DataTransferProtection = "privacy"
	} else {
		// See the comment for this property above.
		options.skipSaslForPrivilegedDatanodePorts = true
	}

	return options
}

// NewClient returns a connected Client for the given options, or an error if
// the client could not be created.
func NewClient(options ClientOptions) (*Client, error) {
	var err error
	if options.KerberosClient != nil && options.KerberosClient.Credentials == nil {
		return nil, errors.New("kerberos enabled, but kerberos client is missing credentials")
	}

	if options.KerberosClient != nil && options.KerberosServicePrincipleName == "" {
		return nil, errors.New("kerberos enabled, but kerberos namenode SPN is not provided")
	}

	namenode, err := rpc.NewNamenodeConnection(
		rpc.NamenodeConnectionOptions{
			Addresses:                    options.Addresses,
			User:                         options.User,
			DialFunc:                     options.NamenodeDialFunc,
			KerberosClient:               options.KerberosClient,
			KerberosServicePrincipleName: options.KerberosServicePrincipleName,
		},
	)

	if err != nil {
		return nil, err
	}

	return &Client{namenode: namenode, options: options}, nil
}

// New returns Client connected to the namenode(s) specified by address, or an
// error if it can't connect. Multiple namenodes can be specified by separating
// them with commas, for example "nn1:9000,nn2:9000".
//
// The user will be the current system user. Any other relevant options
// (including the address(es) of the namenode(s), if an empty string is passed)
// will be loaded from the Hadoop configuration present at HADOOP_CONF_DIR or
// HADOOP_HOME, as specified by hadoopconf.LoadFromEnvironment and
// ClientOptionsFromConf.
//
// Note, however, that New will not attempt any Kerberos authentication; use
// NewClient if you need that.
func New(address string) (*Client, error) {
	conf, err := hadoopconf.LoadFromEnvironment()
	if err != nil {
		return nil, err
	}

	options := ClientOptionsFromConf(conf)
	if address != "" {
		options.Addresses = strings.Split(address, ",")
	}

	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	options.User = u.Username
	return NewClient(options)
}

// User returns the user that the Client is acting under. This is either the
// current system user or the kerberos principal.
func (c *Client) User() string {
	return c.namenode.User
}

// Name returns the unique name that the Client uses in communication
// with namenodes and datanodes.
func (c *Client) Name() string {
	return c.namenode.ClientName
}

// ReadFile reads the file named by filename and returns the contents.
func (c *Client) ReadFile(filename string) ([]byte, error) {
	f, err := c.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return ioutil.ReadAll(f)
}

// CopyToLocal copies the HDFS file specified by src to the local file at dst.
// If dst already exists, it will be overwritten.
func (c *Client) CopyToLocal(src string, dst string) error {
	local, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer local.Close()

	remote, err := c.Open(src)
	if err != nil {
		return err
	}

	_, err = io.Copy(local, remote)
	if err != nil {
		remote.Close()
		return err
	}

	return remote.Close()
}

// CopyToRemote copies the local file specified by src to the HDFS file at dst.
func (c *Client) CopyToRemote(src string, dst string) error {
	local, err := os.Open(src)
	if err != nil {
		return err
	}
	defer local.Close()

	remote, err := c.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(remote, local)
	if err != nil {
		remote.Close()
		return err
	}

	return remote.Close()
}

func (c *Client) fetchDataEncryptionKey() (*hdfs.DataEncryptionKeyProto, error) {
	if c.encryptionKey != nil {
		return c.encryptionKey, nil
	}

	req := &hdfs.GetDataEncryptionKeyRequestProto{}
	resp := &hdfs.GetDataEncryptionKeyResponseProto{}

	err := c.namenode.Execute("getDataEncryptionKey", req, resp)
	if err != nil {
		return nil, err
	}

	c.encryptionKey = resp.GetDataEncryptionKey()
	return c.encryptionKey, nil
}

func (c *Client) wrapDatanodeDial(dc dialContext, token *hadoop.TokenProto) (dialContext, error) {
	wrap := false
	if c.options.DataTransferProtection != "" {
		wrap = true
	} else {
		defaults, err := c.fetchDefaults()
		if err != nil {
			return nil, err
		}

		wrap = defaults.GetEncryptDataTransfer()
	}

	if wrap {
		key, err := c.fetchDataEncryptionKey()
		if err != nil {
			return nil, err
		}

		return (&transfer.SaslDialer{
			DialFunc:                  dc,
			Key:                       key,
			Token:                     token,
			EnforceQop:                c.options.DataTransferProtection,
			SkipSaslOnPrivilegedPorts: c.options.skipSaslForPrivilegedDatanodePorts,
		}).DialContext, nil
	}

	return dc, nil
}

// Close terminates all underlying socket connections to remote server.
func (c *Client) Close() error {
	return c.namenode.Close()
}
