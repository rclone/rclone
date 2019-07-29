package rcd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/rc/rcflags"
	"github.com/ncw/rclone/fs/rc/rcserver"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "rcd <path to files to serve>*",
	Short: `Run rclone listening to remote control commands only.`,
	Long: `
This runs rclone so that it only listens to remote control commands.

This is useful if you are controlling rclone via the rc API.

If you pass in a path to a directory, rclone will serve that directory
for GET requests on the URL passed in.  It will also open the URL in
the browser when rclone is run.

See the [rc documentation](/rc/) for more info on the rc flags.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if rcflags.Opt.Enabled {
			log.Fatalf("Don't supply --rc flag when using rcd")
		}

		// Start the rc
		rcflags.Opt.Enabled = true
		if len(args) > 0 {
			rcflags.Opt.Files = args[0]
		}

		if rcflags.Opt.WebUI {

			// Get the latest release details
			webUiURL, tag, size := GetLatestReleaseURL()

			// Load the file
			exists := exists(tag + ".zip")
			if !exists {
				fmt.Println("Downloading webui binary. Please wait Size :" + strconv.Itoa(size))
				if err := DownloadFile(tag+".zip", webUiURL); err != nil {
					panic(err)
				} else {
					println("Unzipping")
					if err := os.RemoveAll("/webui"); err != nil {
						fmt.Println("No previous downloads to remove")
					}
					if err := Unzip(tag+".zip", "webui"); err != nil {
						panic("Error extracting file")
					}
				}
			} else {
				println("Files already exists. Skipping download")
			}
		}

		s, err := rcserver.Start(&rcflags.Opt)
		if err != nil {
			log.Fatalf("Failed to start remote control: %v", err)
		}
		if s == nil {
			log.Fatal("rc server not configured")
		}

		s.Wait()
	},
}

/**
Get the latest release details of the rclone-webui-react
*/
func GetLatestReleaseURL() (string, string, int) {
	resp, err := http.Get("https://api.github.com/repos/negative0/rclone-webui-react/releases/latest")
	if err != nil {
		panic("Error getting latest release of rclone-webui")
	}
	results := GitHubRequest{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		panic("Could not decode results from http request")
	}

	res := results.Assets[0].BrowserDownloadURL
	tag := results.TagName
	size := results.Assets[0].Size
	//fmt.Println( "URL:" + res)

	return res, tag, size

}

/**
Helper function to download a file from url to the filepath
*/
func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			panic(err)
		}
	}()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

/**
Helper function to unzip a file specified in src to path dest
*/
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
		} else {
			err := os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

// exists returns whether the given file or directory exists
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

/**
Map the GitHub API request to structure
*/
type GitHubRequest struct {
	URL string `json:"url"`

	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	TagName     string    `json:"tag_name"`
	Assets      []struct {
		URL                string    `json:"url"`
		ID                 int       `json:"id"`
		NodeID             string    `json:"node_id"`
		Name               string    `json:"name"`
		Label              string    `json:"label"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadURL string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballURL string `json:"tarball_url"`
	ZipballURL string `json:"zipball_url"`
	Body       string `json:"body"`
}
