package lsjson

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/ncw/rclone/backend/crypt"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/ls/lshelp"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/operations"
	"github.com/ncw/rclone/fs/walk"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	recurse       bool
	showHash      bool
	showEncrypted bool
	showOrigIDs   bool
	noModTime     bool
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&recurse, "recursive", "R", false, "Recurse into the listing.")
	commandDefintion.Flags().BoolVarP(&showHash, "hash", "", false, "Include hashes in the output (may take longer).")
	commandDefintion.Flags().BoolVarP(&noModTime, "no-modtime", "", false, "Don't read the modification time (can speed things up).")
	commandDefintion.Flags().BoolVarP(&showEncrypted, "encrypted", "M", false, "Show the encrypted names.")
	commandDefintion.Flags().BoolVarP(&showOrigIDs, "original", "", false, "Show the ID of the underlying Object.")
}

// lsJSON in the struct which gets marshalled for each line
type lsJSON struct {
	Path      string
	Name      string
	Encrypted string `json:",omitempty"`
	Size      int64
	MimeType  string    `json:",omitempty"`
	ModTime   Timestamp //`json:",omitempty"`
	IsDir     bool
	Hashes    map[string]string `json:",omitempty"`
	ID        string            `json:",omitempty"`
	OrigID    string            `json:",omitempty"`
}

// Timestamp a time in RFC3339 format with Nanosecond precision secongs
type Timestamp time.Time

// MarshalJSON turns a Timestamp into JSON
func (t Timestamp) MarshalJSON() (out []byte, err error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + tt.Format(time.RFC3339Nano) + `"`), nil
}

var commandDefintion = &cobra.Command{
	Use:   "lsjson remote:path",
	Short: `List directories and objects in the path in JSON format.`,
	Long: `List directories and objects in the path in JSON format.

The output is an array of Items, where each Item looks like this

   {
      "Hashes" : {
         "SHA-1" : "f572d396fae9206628714fb2ce00f72e94f2258f",
         "MD5" : "b1946ac92492d2347c6235b4d2611184",
         "DropboxHash" : "ecb65bb98f9d905b70458986c39fcbad7715e5f2fcc3b1f07767d7c83e2438cc"
      },
      "ID": "y2djkhiujf83u33",
      "OrigID": "UYOJVTUW00Q1RzTDA",
      "IsDir" : false,
      "MimeType" : "application/octet-stream",
      "ModTime" : "2017-05-31T16:15:57.034468261+01:00",
      "Name" : "file.txt",
      "Encrypted" : "v0qpsdq8anpci8n929v3uu9338",
      "Path" : "full/path/goes/here/file.txt",
      "Size" : 6
   }

If --hash is not specified the Hashes property won't be emitted.

If --no-modtime is specified then ModTime will be blank.

If --encrypted is not specified the Encrypted won't be emitted.

The Path field will only show folders below the remote path being listed.
If "remote:path" contains the file "subfolder/file.txt", the Path for "file.txt"
will be "subfolder/file.txt", not "remote:path/subfolder/file.txt".
When used without --recursive the Path will always be the same as Name.

The time is in RFC3339 format with nanosecond precision.

The whole output can be processed as a JSON blob, or alternatively it
can be processed line by line as each item is written one to a line.
` + lshelp.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		var cipher crypt.Cipher
		if showEncrypted {
			fsInfo, _, _, config, err := fs.ConfigFs(args[0])
			if err != nil {
				log.Fatalf(err.Error())
			}
			if fsInfo.Name != "crypt" {
				log.Fatalf("The remote needs to be of type \"crypt\"")
			}
			cipher, err = crypt.NewCipher(config)
			if err != nil {
				log.Fatalf(err.Error())
			}
		}
		cmd.Run(false, false, command, func() error {
			fmt.Println("[")
			first := true
			err := walk.Walk(fsrc, "", false, operations.ConfigMaxDepth(recurse), func(dirPath string, entries fs.DirEntries, err error) error {
				if err != nil {
					fs.CountError(err)
					fs.Errorf(dirPath, "error listing: %v", err)
					return nil
				}
				for _, entry := range entries {
					item := lsJSON{
						Path:     entry.Remote(),
						Name:     path.Base(entry.Remote()),
						Size:     entry.Size(),
						MimeType: fs.MimeTypeDirEntry(entry),
					}
					if !noModTime {
						item.ModTime = Timestamp(entry.ModTime())
					}
					if cipher != nil {
						switch entry.(type) {
						case fs.Directory:
							item.Encrypted = cipher.EncryptDirName(path.Base(entry.Remote()))
						case fs.Object:
							item.Encrypted = cipher.EncryptFileName(path.Base(entry.Remote()))
						default:
							fs.Errorf(nil, "Unknown type %T in listing", entry)
						}
					}
					if do, ok := entry.(fs.IDer); ok {
						item.ID = do.ID()
					}
					if showOrigIDs {
						cur := entry
						for {
							u, ok := cur.(fs.ObjectUnWrapper)
							if !ok {
								break // not a wrapped object, use current id
							}
							next := u.UnWrap()
							if next == nil {
								break // no base object found, use current id
							}
							cur = next
						}
						if do, ok := cur.(fs.IDer); ok {
							item.OrigID = do.ID()
						}
					}
					switch x := entry.(type) {
					case fs.Directory:
						item.IsDir = true
					case fs.Object:
						item.IsDir = false
						if showHash {
							item.Hashes = make(map[string]string)
							for _, hashType := range x.Fs().Hashes().Array() {
								hash, err := x.Hash(hashType)
								if err != nil {
									fs.Errorf(x, "Failed to read hash: %v", err)
								} else if hash != "" {
									item.Hashes[hashType.String()] = hash
								}
							}
						}
					default:
						fs.Errorf(nil, "Unknown type %T in listing", entry)
					}
					out, err := json.Marshal(item)
					if err != nil {
						return errors.Wrap(err, "failed to marshal list object")
					}
					if first {
						first = false
					} else {
						fmt.Print(",\n")
					}
					_, err = os.Stdout.Write(out)
					if err != nil {
						return errors.Wrap(err, "failed to write to output")
					}

				}
				return nil
			})
			if err != nil {
				return errors.Wrap(err, "error listing JSON")
			}
			if !first {
				fmt.Println()
			}
			fmt.Println("]")
			return nil
		})
	},
}
