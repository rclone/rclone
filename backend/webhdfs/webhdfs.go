// Package webhdfs provides an interface to remote storage systems
// exposing the Hadoop WebHDFS REST API.
//
// See: https://hadoop.apache.org/docs/r1.0.4/webhdfs.html
package webhdfs

import (
	"path"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/encoder"
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "webhdfs",
		Description: "Hadoop distributed file system over HTTP (WebHDFS)",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "url",
			Help: "URL of the WebHDFS namenode (or HttpFS server).\n\n" +
				"This is the base URL rclone connects to for every request, so it must be\n" +
				"reachable from the machine running rclone and point at the WebHDFS REST\n" +
				"endpoint of the namenode (or an HttpFS gateway acting as a proxy for it).\n\n" +
				"E.g. \"http://namenode:9870\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name: "username",
			Help: "Hadoop user name.\n\n" +
				"Set this when security is off - it is sent as the \"user.name\" query\n" +
				"parameter on every request and identifies which Hadoop user rclone acts\n" +
				"as for permission checks. Leave blank when Kerberos or another\n" +
				"authentication mechanism is used instead.",
			Examples: []fs.OptionExample{{
				Value: "root",
				Help:  "Connect to WebHDFS as root.",
			}},
			Sensitive: true,
		}, {
			Name: "password",
			Help: "Password.\n\n" +
				"Set this together with username to use HTTP Basic Authentication\n" +
				"instead of (or in addition to) the \"user.name\" parameter, for example\n" +
				"when the namenode or HttpFS gateway is fronted by a proxy that enforces\n" +
				"Basic Auth.",
			IsPassword: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default:  (encoder.Display | encoder.EncodeInvalidUtf8 | encoder.EncodeColon),
		}},
	}
	fs.Register(fsi)
}

// Options for this backend
type Options struct {
	URL      string               `config:"url"`
	Username string               `config:"username"`
	Password string               `config:"password"`
	Enc      encoder.MultiEncoder `config:"encoding"`
}

// xPath makes correct file path with leading '/'
func xPath(root string, tail string) string {
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	return path.Join(root, tail)
}
