package dlna

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anacrolix/dms/dlna"
	"github.com/anacrolix/dms/upnp"
	"github.com/rclone/rclone/cmd/serve/dlna/upnpav"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

type contentDirectoryService struct {
	*server
	upnp.Eventing
}

func (cds *contentDirectoryService) updateIDString() string {
	return fmt.Sprintf("%d", uint32(os.Getpid()))
}

var mediaMimeTypeRegexp = regexp.MustCompile("^(video|audio|image)/")

// Turns the given entry and DMS host into a UPnP object. A nil object is
// returned if the entry is not of interest.
func (cds *contentDirectoryService) cdsObjectToUpnpavObject(cdsObject object, fileInfo vfs.Node, resources vfs.Nodes, host string) (ret interface{}, err error) {
	obj := upnpav.Object{
		ID:         cdsObject.ID(),
		Restricted: 1,
		ParentID:   cdsObject.ParentID(),
	}

	if fileInfo.IsDir() {
		defaultChildCount := 1
		obj.Class = "object.container.storageFolder"
		obj.Title = fileInfo.Name()
		return upnpav.Container{
			Object:     obj,
			ChildCount: &defaultChildCount,
		}, nil
	}

	if !fileInfo.Mode().IsRegular() {
		return
	}

	// Read the mime type from the fs.Object if possible,
	// otherwise fall back to working out what it is from the file path.
	var mimeType string
	if o, ok := fileInfo.DirEntry().(fs.Object); ok {
		mimeType = fs.MimeType(context.TODO(), o)
		// If backend doesn't know what the mime type is then
		// try getting it from the file name
		if mimeType == "application/octet-stream" {
			mimeType = fs.MimeTypeFromName(fileInfo.Name())
		}
	} else {
		mimeType = fs.MimeTypeFromName(fileInfo.Name())
	}

	mediaType := mediaMimeTypeRegexp.FindStringSubmatch(mimeType)
	if mediaType == nil {
		return
	}

	obj.Class = "object.item." + mediaType[1] + "Item"
	obj.Title = fileInfo.Name()
	obj.Date = upnpav.Timestamp{Time: fileInfo.ModTime()}

	item := upnpav.Item{
		Object: obj,
		Res:    make([]upnpav.Resource, 0, 1),
	}

	item.Res = append(item.Res, upnpav.Resource{
		URL: (&url.URL{
			Scheme: "http",
			Host:   host,
			Path:   path.Join(resPath, cdsObject.Path),
		}).String(),
		ProtocolInfo: fmt.Sprintf("http-get:*:%s:%s", mimeType, dlna.ContentFeatures{
			SupportRange: true,
		}.String()),
		Size: uint64(fileInfo.Size()),
	})

	for _, resource := range resources {
		subtitleURL := (&url.URL{
			Scheme: "http",
			Host:   host,
			Path:   path.Join(resPath, resource.Path()),
		}).String()

		// Read the mime type from the fs.Object if possible,
		// otherwise fall back to working out what it is from the file path.
		var mimeType string
		if o, ok := resource.DirEntry().(fs.Object); ok {
			mimeType = fs.MimeType(context.TODO(), o)
			// If backend doesn't know what the mime type is then
			// try getting it from the file name
			if mimeType == "application/octet-stream" {
				mimeType = fs.MimeTypeFromName(resource.Name())
			}
		} else {
			mimeType = fs.MimeTypeFromName(resource.Name())
		}

		item.Res = append(item.Res, upnpav.Resource{
			URL:          subtitleURL,
			ProtocolInfo: fmt.Sprintf("http-get:*:%s:*", mimeType),
		})
	}

	ret = item
	return
}

// Returns all the upnpav objects in a directory.
func (cds *contentDirectoryService) readContainer(o object, host string) (ret []interface{}, err error) {
	node, err := cds.vfs.Stat(o.Path)
	if err != nil {
		return
	}

	if !node.IsDir() {
		err = errors.New("not a directory")
		return
	}

	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		err = errors.New("failed to list directory")
		return
	}

	// if there's a "Subs" child directory, add its children to the list as well,
	// so mediaWithResources is able to find them.
	for _, node := range dirEntries {
		if strings.EqualFold(node.Name(), "Subs") && node.IsDir() {
			subtitleDir := node.(*vfs.Dir)
			subtitleEntries, err := subtitleDir.ReadDirAll()
			if err != nil {
				err = errors.New("failed to list subtitle directory")
				return nil, err
			}
			dirEntries = append(dirEntries, subtitleEntries...)
		}
	}

	dirEntries, mediaResources := mediaWithResources(dirEntries)
	for _, de := range dirEntries {
		child := object{
			path.Join(o.Path, de.Name()),
		}
		obj, err := cds.cdsObjectToUpnpavObject(child, de, mediaResources[de], host)
		if err != nil {
			fs.Errorf(cds, "error with %s: %s", child.FilePath(), err)
			continue
		}
		if obj == nil {
			fs.Debugf(cds, "unrecognized file type: %s", de)
			continue
		}
		ret = append(ret, obj)
	}

	return
}

// Given a list of nodes, separate them into potential media items and any associated resources (external subtitles,
// for example.)
//
// The result is a slice of potential media nodes (in their original order) and a map containing associated
// resources nodes of each media node, if any.
func mediaWithResources(nodes vfs.Nodes) (vfs.Nodes, map[vfs.Node]vfs.Nodes) {
	media, mediaResources := vfs.Nodes{}, make(map[vfs.Node]vfs.Nodes)

	// First, separate out the subtitles and media into maps, keyed by their lowercase base names.
	mediaByName, subtitlesByName := make(map[string]vfs.Nodes), make(map[string]vfs.Nodes)
	for _, node := range nodes {
		baseName, ext := splitExt(strings.ToLower(node.Name()))
		switch ext {
		case ".srt", ".ass", ".ssa", ".sub", ".idx", ".sup", ".jss", ".txt", ".usf", ".cue", ".vtt", ".css":
			// .idx should be with .sub, .css should be with vtt otherwise they should be culled,
			// and their mimeTypes are not consistent, but anyway these negatives don't throw errors.
			subtitlesByName[baseName] = append(subtitlesByName[baseName], node)
		default:
			mediaByName[baseName] = append(mediaByName[baseName], node)
			media = append(media, node)
		}
	}

	// Find the associated media file for each subtitle
	for baseName, nodes := range subtitlesByName {
		// Find a media file with the same basename (video.mp4 for video.srt)
		mediaNodes, found := mediaByName[baseName]
		if !found {
			// Or basename of the basename (video.mp4 for video.en.srt)
			baseName, _ := splitExt(baseName)
			mediaNodes, found = mediaByName[baseName]
		}

		// Just advise if no match found
		if !found {
			fs.Infof(nodes, "could not find associated media for subtitle: %s", baseName)
			fs.Infof(mediaByName, "mediaByName is this, baseName is %s", baseName)
			continue
		}

		// Associate with all potential media nodes
		fs.Debugf(mediaNodes, "associating subtitle: %s", baseName)
		for _, mediaNode := range mediaNodes {
			mediaResources[mediaNode] = append(mediaResources[mediaNode], nodes...)
		}
	}

	return media, mediaResources
}

type browse struct {
	ObjectID       string
	BrowseFlag     string
	Filter         string
	StartingIndex  int
	RequestedCount int
}

// ContentDirectory object from ObjectID.
func (cds *contentDirectoryService) objectFromID(id string) (o object, err error) {
	o.Path, err = url.QueryUnescape(id)
	if err != nil {
		return
	}
	if o.Path == "0" {
		o.Path = "/"
	}
	o.Path = path.Clean(o.Path)
	if !path.IsAbs(o.Path) {
		err = fmt.Errorf("bad ObjectID %v", o.Path)
		return
	}
	return
}

func (cds *contentDirectoryService) Handle(action string, argsXML []byte, r *http.Request) (map[string]string, error) {
	host := r.Host

	switch action {
	case "GetSystemUpdateID":
		return map[string]string{
			"Id": cds.updateIDString(),
		}, nil
	case "GetSortCapabilities":
		return map[string]string{
			"SortCaps": "dc:title",
		}, nil
	case "Browse":
		var browse browse
		if err := xml.Unmarshal(argsXML, &browse); err != nil {
			return nil, err
		}
		obj, err := cds.objectFromID(browse.ObjectID)
		if err != nil {
			return nil, upnp.Errorf(upnpav.NoSuchObjectErrorCode, "%s", err.Error())
		}
		switch browse.BrowseFlag {
		case "BrowseDirectChildren":
			objs, err := cds.readContainer(obj, host)
			if err != nil {
				return nil, upnp.Errorf(upnpav.NoSuchObjectErrorCode, "%s", err.Error())
			}
			totalMatches := len(objs)
			objs = objs[func() (low int) {
				low = browse.StartingIndex
				if low > len(objs) {
					low = len(objs)
				}
				return
			}():]
			if browse.RequestedCount != 0 && browse.RequestedCount < len(objs) {
				objs = objs[:browse.RequestedCount]
			}
			result, err := xml.Marshal(objs)
			if err != nil {
				return nil, err
			}
			return map[string]string{
				"TotalMatches":   fmt.Sprint(totalMatches),
				"NumberReturned": fmt.Sprint(len(objs)),
				"Result":         didlLite(string(result)),
				"UpdateID":       cds.updateIDString(),
			}, nil
		case "BrowseMetadata":
			node, err := cds.vfs.Stat(obj.Path)
			if err != nil {
				return nil, err
			}
			// TODO: External subtitles won't appear in the metadata here, but probably should.
			upnpObject, err := cds.cdsObjectToUpnpavObject(obj, node, vfs.Nodes{}, host)
			if err != nil {
				return nil, err
			}
			result, err := xml.Marshal(upnpObject)
			if err != nil {
				return nil, err
			}
			return map[string]string{
				"TotalMatches":   "1",
				"NumberReturned": "1",
				"Result":         didlLite(string(result)),
				"UpdateID":       cds.updateIDString(),
			}, nil
		default:
			return nil, upnp.Errorf(upnp.ArgumentValueInvalidErrorCode, "unhandled browse flag: %v", browse.BrowseFlag)
		}
	case "GetSearchCapabilities":
		return map[string]string{
			"SearchCaps": "",
		}, nil
	// Samsung Extensions
	case "X_GetFeatureList":
		return map[string]string{
			"FeatureList": `<Features xmlns="urn:schemas-upnp-org:av:avs" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:schemas-upnp-org:av:avs http://www.upnp.org/schemas/av/avs.xsd">
	<Feature name="samsung.com_BASICVIEW" version="1">
		<container id="0" type="object.item.imageItem"/>
		<container id="0" type="object.item.audioItem"/>
		<container id="0" type="object.item.videoItem"/>
	</Feature>
</Features>`}, nil
	case "X_SetBookmark":
		// just ignore
		return map[string]string{}, nil
	default:
		return nil, upnp.InvalidActionError
	}
}

// Represents a ContentDirectory object.
type object struct {
	Path string // The cleaned, absolute path for the object relative to the server.
}

// Returns the actual local filesystem path for the object.
func (o *object) FilePath() string {
	return filepath.FromSlash(o.Path)
}

// Returns the ObjectID for the object. This is used in various ContentDirectory actions.
func (o object) ID() string {
	if !path.IsAbs(o.Path) {
		fs.Panicf(nil, "Relative object path: %s", o.Path)
	}
	if len(o.Path) == 1 {
		return "0"
	}
	return url.QueryEscape(o.Path)
}

func (o *object) IsRoot() bool {
	return o.Path == "/"
}

// Returns the object's parent ObjectID. Fortunately it can be deduced from the
// ObjectID (for now).
func (o object) ParentID() string {
	if o.IsRoot() {
		return "-1"
	}
	o.Path = path.Dir(o.Path)
	return o.ID()
}
