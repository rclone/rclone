// +build !plan9

package hdfs

import (
	"path"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/encoder"
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "hdfs",
		Description: "Hadoop distributed file system",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "namenode",
			Help:     "hadoop name node and port",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "namenode:8020",
				Help:  "Connect to host namenode at port 8020",
			}},
		}, {
			Name:     "username",
			Help:     "hadoop user name",
			Required: false,
			Examples: []fs.OptionExample{{
				Value: "root",
				Help:  "Connect to hdfs as root",
			}},
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
	Namenode string               `config:"namenode"`
	Username string               `config:"username"`
	Enc      encoder.MultiEncoder `config:"encoding"`
}

// xPath make correct file path with leading '/'
func xPath(root string, tail string) string {
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	return path.Join(root, tail)
}
