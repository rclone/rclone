// Define the Web GUI helpers

package webgui

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
)

// GetLatestReleaseURL returns the latest release details of the rclone-webui-react
func GetLatestReleaseURL(fetchURL string) (string, string, int, error) {
	resp, err := http.Get(fetchURL)
	if err != nil {
		return "", "", 0, errors.Wrap(err, "failed getting latest release of rclone-webui")
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, errors.Errorf("bad HTTP status %d (%s) when fetching %s", resp.StatusCode, resp.Status, fetchURL)
	}
	results := gitHubRequest{}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", "", 0, errors.Wrap(err, "could not decode results from http request")
	}
	if len(results.Assets) < 1 {
		return "", "", 0, errors.New("could not find an asset in the release. " +
			"check if asset was successfully added in github release assets")
	}
	res := results.Assets[0].BrowserDownloadURL
	tag := results.TagName
	size := results.Assets[0].Size

	return res, tag, size, nil
}

// CheckAndDownloadWebGUIRelease is a helper function to download and setup latest release of rclone-webui-react
func CheckAndDownloadWebGUIRelease(checkUpdate bool, forceUpdate bool, fetchURL string, cacheDir string) (err error) {
	cachePath := filepath.Join(cacheDir, "webgui")
	tagPath := filepath.Join(cachePath, "tag")
	extractPath := filepath.Join(cachePath, "current")

	extractPathExist, extractPathStat, err := exists(extractPath)
	if err != nil {
		return err
	}

	if extractPathExist && !extractPathStat.IsDir() {
		return errors.New("Web GUI path exists, but is a file instead of folder. Please check the path " + extractPath)
	}

	// if the old file exists does not exist or forced update is enforced.
	// TODO: Add hashing to check integrity of the previous update.
	if !extractPathExist || checkUpdate || forceUpdate {
		// Get the latest release details
		WebUIURL, tag, size, err := GetLatestReleaseURL(fetchURL)
		if err != nil {
			return err
		}

		dat, err := ioutil.ReadFile(tagPath)
		if err == nil && string(dat) == tag {
			fs.Logf(nil, "No update to Web GUI available.")
			if !forceUpdate {
				return nil
			}
			fs.Logf(nil, "Force update the Web GUI binary.")
		}

		zipName := tag + ".zip"
		zipPath := filepath.Join(cachePath, zipName)

		cachePathExist, cachePathStat, _ := exists(cachePath)
		if !cachePathExist {
			if err := os.MkdirAll(cachePath, 0755); err != nil {
				return errors.New("Error creating cache directory: " + cachePath)
			}
		}

		if cachePathExist && !cachePathStat.IsDir() {
			return errors.New("Web GUI path is a file instead of folder. Please check it " + extractPath)
		}

		fs.Logf(nil, "A new release for gui is present at "+WebUIURL)
		fs.Logf(nil, "Downloading webgui binary. Please wait. [Size: %s, Path :  %s]\n", strconv.Itoa(size), zipPath)

		// download the zip from latest url
		err = DownloadFile(zipPath, WebUIURL)
		if err != nil {
			return err
		}

		err = os.RemoveAll(extractPath)
		if err != nil {
			fs.Logf(nil, "No previous downloads to remove")
		}
		fs.Logf(nil, "Unzipping webgui binary")

		err = Unzip(zipPath, extractPath)
		if err != nil {
			return err
		}

		err = os.RemoveAll(zipPath)
		if err != nil {
			fs.Logf(nil, "Downloaded ZIP cannot be deleted")
		}

		err = ioutil.WriteFile(tagPath, []byte(tag), 0644)
		if err != nil {
			fs.Infof(nil, "Cannot write tag file. You may be required to redownload the binary next time.")
		}
	} else {
		fs.Logf(nil, "Web GUI exists. Update skipped.")
	}

	return nil
}

// DownloadFile is a helper function to download a file from url to the filepath
func DownloadFile(filepath string, url string) (err error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("bad HTTP status %d (%s) when fetching %s", resp.StatusCode, resp.Status, url)
	}

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

// Unzip is a helper function to Unzip a file specified in src to path dest
func Unzip(src, dest string) (err error) {
	dest = filepath.Clean(dest) + string(os.PathSeparator)

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
		path := filepath.Join(dest, f.Name)
		// Check for Zip Slip: https://github.com/rclone/rclone/issues/3529
		if !strings.HasPrefix(path, dest) {
			return fmt.Errorf("%s: illegal file path", path)
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer fs.CheckClose(rc, &err)

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

func exists(path string) (existence bool, stat os.FileInfo, err error) {
	stat, err = os.Stat(path)
	if err == nil {
		return true, stat, nil
	}
	if os.IsNotExist(err) {
		return false, nil, nil
	}
	return false, stat, err
}

// CreatePathIfNotExist creates the path to a folder if it does not exist
func CreatePathIfNotExist(path string) (err error) {
	exists, stat, _ := exists(path)
	if !exists {
		if err := os.MkdirAll(path, 0755); err != nil {
			return errors.New("Error creating : " + path)
		}
	}

	if exists && !stat.IsDir() {
		return errors.New("Path is a file instead of folder. Please check it " + path)
	}

	return nil
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
