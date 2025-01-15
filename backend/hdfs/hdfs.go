//go:build !plan9

// Package hdfs provides an interface to the HDFS storage system.
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
			Name:      "namenode",
			Help:      "Hadoop name nodes and ports.\n\nE.g. \"namenode-1:8020,namenode-2:8020,...\" to connect to host namenodes at port 8020.",
			Required:  true,
			Sensitive: true,
			Default:   fs.CommaSepList{},
		}, {
			Name: "username",
			Help: "Hadoop user name.",
			Examples: []fs.OptionExample{{
				Value: "root",
				Help:  "Connect to hdfs as root.",
			}},
			Sensitive: true,
		}, {
			Name: "service_principal_name",
			Help: `Kerberos service principal name for the namenode.

Enables KERBEROS authentication. Specifies the Service Principal Name
(SERVICE/FQDN) for the namenode. E.g. \"hdfs/namenode.hadoop.docker\"
for namenode running as service 'hdfs' with FQDN 'namenode.hadoop.docker'.`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "data_transfer_protection",
			Help: `Kerberos data transfer protection: authentication|integrity|privacy.

Specifies whether or not authentication, data signature integrity
checks, and wire encryption are required when communicating with
the datanodes. Possible values are 'authentication', 'integrity'
and 'privacy'. Used only with KERBEROS enabled.`,
			Examples: []fs.OptionExample{{
				Value: "privacy",
				Help:  "Ensure authentication, integrity and encryption enabled.",
			}},
			Advanced: true,
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
	Namenode               fs.CommaSepList      `config:"namenode"`
	Username               string               `config:"username"`
	ServicePrincipalName   string               `config:"service_principal_name"`
	DataTransferProtection string               `config:"data_transfer_protection"`
	Enc                    encoder.MultiEncoder `config:"encoding"`
}

// xPath make correct file path with leading '/'
func xPath(root string, tail string) string {
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	return path.Join(root, tail)
}
