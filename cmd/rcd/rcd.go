package rcd

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/rc/rcflags"
	"github.com/rclone/rclone/fs/rc/rcserver"
	"github.com/rclone/rclone/lib/errors"
	"github.com/rclone/rclone/lib/random"
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
			if err := checkRelease(rcflags.Opt.WebGUIUpdate); err != nil {
				log.Fatalf("Error while fetching the latest release of rclone-webui-react %v", err)
			}
			if rcflags.Opt.NoAuth {
				rcflags.Opt.NoAuth = false
				fs.Infof(nil, "Cannot run web-gui without authentication, using default auth")
			}
			if rcflags.Opt.HTTPOptions.BasicUser == "" {
				rcflags.Opt.HTTPOptions.BasicUser = "gui"
				fs.Infof(nil, "Using default username: %s \n", rcflags.Opt.HTTPOptions.BasicUser)
			}
			if rcflags.Opt.HTTPOptions.BasicPass == "" {
				randomPass, err := random.Password(128)
				if err != nil {
					log.Fatalf("Failed to make password: %v", err)
				}
				rcflags.Opt.HTTPOptions.BasicPass = randomPass
				fs.Infof(nil, "No password specified. Using random password: %s \n", randomPass)
			}
			rcflags.Opt.Serve = true
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

//checkRelease is a helper function to download and setup latest release of rclone-webui-react
func checkRelease(shouldUpdate bool) (err error) {
	cachePath := filepath.Join(config.CacheDir, "webgui")
	extractPath := filepath.Join(cachePath, "current")
	oldUpdateExists := exists(extractPath)

	// if the old file exists does not exist or forced update is enforced.
	// TODO: Add hashing to check integrity of the previous update.
	if !oldUpdateExists || shouldUpdate {
		// Get the latest release details
		WebUIURL, tag, size, err := getLatestReleaseURL()
		if err != nil {
			return err
		}

		zipName := tag + ".zip"
		zipPath := filepath.Join(cachePath, zipName)

		if !exists(cachePath) {
			if err := os.MkdirAll(cachePath, 0755); err != nil {
				fs.Logf(nil, "Error creating cache directory: %s", cachePath)
				return err
			}
		}

		fs.Logf(nil, "A new release for gui is present at "+WebUIURL)
		fs.Logf(nil, "Downloading webgui binary. Please wait. [Size: %s, Path :  %s]\n", strconv.Itoa(size), zipPath)

		// download the zip from latest url
		err = downloadFile(zipPath, WebUIURL)
		if err != nil {
			return err
		}
		err = os.RemoveAll(extractPath)
		if err != nil {
			fs.Logf(nil, "No previous downloads to remove")
		}
		fs.Logf(nil, "Unzipping")
		err = unzip(zipPath, extractPath)
		if err != nil {
			return err
		}

	} else {
		fs.Logf(nil, "Required files exist. Skipping download")
	}
	return nil
}

// getLatestReleaseURL returns the latest release details of the rclone-webui-react
func getLatestReleaseURL() (string, string, int, error) {
	resp, err := http.Get(rcflags.Opt.WebGUIFetchURL)
	if err != nil {
		return "", "", 0, errors.New("Error getting latest release of rclone-webui")
	}
	results := gitHubRequest{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", "", 0, errors.New("Could not decode results from http request")
	}

	res := results.Assets[0].BrowserDownloadURL
	tag := results.TagName
	size := results.Assets[0].Size
	//fmt.Println( "URL:" + res)

	return res, tag, size, nil

}

// downloadFile is a helper function to download a file from url to the filepath
func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer fs.CheckClose(out, &err)

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// unzip is a helper function to unzip a file specified in src to path dest
func unzip(src, dest string) (err error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer fs.CheckClose(r, &err)

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer fs.CheckClose(rc, &err)

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			defer fs.CheckClose(f, &err)

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

// gitHubRequest Maps the GitHub API request to structure
type gitHubRequest struct {
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
