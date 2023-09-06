package hdfs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"google.golang.org/protobuf/proto"
)

var errXAttrKeysNotFound = errors.New("one or more keys not found")

const createAndReplace = 3

// ListXAttrs returns a list of all extended attributes for the given path.
// The returned keys will be in the form
func (c *Client) ListXAttrs(name string) (map[string]string, error) {
	req := &hdfs.ListXAttrsRequestProto{Src: proto.String(name)}
	resp := &hdfs.ListXAttrsResponseProto{}

	err := c.namenode.Execute("listXAttrs", req, resp)
	if err != nil {
		return nil, &os.PathError{"list xattrs", name, interpretException(err)}
	}

	return xattrMap(resp.GetXAttrs()), nil
}

// GetXAttrs returns the extended attributes for the given path and list of
// keys. The keys should be prefixed by namespace, e.g. user.foo or trusted.bar.
func (c *Client) GetXAttrs(name string, keys ...string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}

	req := &hdfs.GetXAttrsRequestProto{Src: proto.String(name)}
	for _, key := range keys {
		ns, rest, err := splitKey(key)
		if err != nil {
			return nil, &os.PathError{"get xattrs", name, err}
		}

		req.XAttrs = append(req.XAttrs, &hdfs.XAttrProto{
			Namespace: ns,
			Name:      proto.String(rest),
		})
	}
	resp := &hdfs.GetXAttrsResponseProto{}

	err := c.namenode.Execute("getXAttrs", req, resp)
	if err != nil {
		if isKeyNotFound(err) {
			return nil, &os.PathError{"get xattrs", name, errXAttrKeysNotFound}
		}

		return nil, &os.PathError{"get xattrs", name, interpretException(err)}
	}

	return xattrMap(resp.GetXAttrs()), nil
}

// SetXAttr sets an extended attribute for the given path and key. If the
// attribute doesn't exist, it will be created.
func (c *Client) SetXAttr(name, key, value string) error {
	resp := &hdfs.SetXAttrResponseProto{}

	ns, rest, err := splitKey(key)
	if err != nil {
		return &os.PathError{"set xattr", name, err}
	}

	req := &hdfs.SetXAttrRequestProto{
		Src: proto.String(name),
		XAttr: &hdfs.XAttrProto{
			Namespace: ns.Enum(),
			Name:      proto.String(rest),
			Value:     []byte(value),
		},
		Flag: proto.Uint32(createAndReplace),
	}

	err = c.namenode.Execute("setXAttr", req, resp)
	if err != nil {
		return &os.PathError{"set xattr", name, interpretException(err)}
	}

	return nil
}

// RemoveXAttr unsets an extended attribute for the given path and key. It
// returns an error if the attribute doesn't already exist.
func (c *Client) RemoveXAttr(name, key string) error {
	ns, rest, err := splitKey(key)
	if err != nil {
		return &os.PathError{"remove xattr", name, err}
	}

	req := &hdfs.RemoveXAttrRequestProto{
		Src: proto.String(name),
		XAttr: &hdfs.XAttrProto{
			Namespace: ns,
			Name:      proto.String(rest),
		},
	}
	resp := &hdfs.RemoveXAttrResponseProto{}

	err = c.namenode.Execute("removeXAttr", req, resp)
	if err != nil {
		if isKeyNotFound(err) {
			return &os.PathError{"remove xattr", name, errXAttrKeysNotFound}
		}

		return &os.PathError{"remove xattr", name, interpretException(err)}
	}

	return nil
}

func splitKey(key string) (*hdfs.XAttrProto_XAttrNamespaceProto, string, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("invalid key: '%s'", key)
	}

	var ns hdfs.XAttrProto_XAttrNamespaceProto
	switch strings.ToLower(parts[0]) {
	case "user":
		ns = hdfs.XAttrProto_USER
	case "trusted":
		ns = hdfs.XAttrProto_TRUSTED
	case "system":
		ns = hdfs.XAttrProto_SYSTEM
	case "security":
		ns = hdfs.XAttrProto_SECURITY
	case "raw":
		ns = hdfs.XAttrProto_RAW
	default:
		return nil, "", fmt.Errorf("invalid key namespace: '%s'", parts[0])
	}

	return ns.Enum(), parts[1], nil
}

func xattrMap(attrs []*hdfs.XAttrProto) map[string]string {
	m := make(map[string]string)
	for _, xattr := range attrs {
		key := fmt.Sprintf("%s.%s",
			strings.ToLower(xattr.GetNamespace().String()), xattr.GetName())
		m[key] = string(xattr.GetValue())
	}

	return m
}

func isKeyNotFound(err error) bool {
	if remoteErr, ok := err.(Error); ok {
		if strings.HasPrefix(remoteErr.Message(),
			"At least one of the attributes provided was not found") {
			return true
		}

		if strings.HasPrefix(remoteErr.Message(),
			"No matching attributes found for remove operation") {
			return true
		}
	}

	return false
}
