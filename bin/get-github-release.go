// +build ignore

// Get the latest release from a github project
//
// If GITHUB_USER and GITHUB_TOKEN are set then these will be used to
// authenticate the request which is useful to avoid rate limits.

package main

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/net/html"
	"golang.org/x/sys/unix"
)

var (
	// Flags
	install = flag.Bool("install", false, "Install the downloaded package using sudo dpkg -i.")
	extract = flag.String("extract", "", "Extract the named executable from the .tar.gz and install into bindir.")
	bindir  = flag.String("bindir", defaultBinDir(), "Directory to install files downloaded with -extract.")
	useAPI  = flag.Bool("use-api", false, "Use the API for finding the release instead of scraping the page.")
	// Globals
	matchProject = regexp.MustCompile(`^([\w-]+)/([\w-]+)$`)
	osAliases    = map[string][]string{
		"darwin": {"macos", "osx"},
	}
	archAliases = map[string][]string{
		"amd64": {"x86_64"},
	}
)

// A github release
//
// Made by pasting the JSON into https://mholt.github.io/json-to-go/
type Release struct {
	URL             string `json:"url"`
	AssetsURL       string `json:"assets_url"`
	UploadURL       string `json:"upload_url"`
	HTMLURL         string `json:"html_url"`
	ID              int    `json:"id"`
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Name            string `json:"name"`
	Draft           bool   `json:"draft"`
	Author          struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		URL      string `json:"url"`
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
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

// checks if a path has write access
func writable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}

// Directory to install releases in by default
//
// Find writable directories on $PATH.  Use $GOPATH/bin if that is on
// the path and writable or use the first writable directory which is
// in $HOME or failing that the first writable directory.
//
// Returns "" if none of the above were found
func defaultBinDir() string {
	home := os.Getenv("HOME")
	var (
		bin       string
		homeBin   string
		goHomeBin string
		gopath    = os.Getenv("GOPATH")
	)
	for _, dir := range strings.Split(os.Getenv("PATH"), ":") {
		if writable(dir) {
			if strings.HasPrefix(dir, home) {
				if homeBin != "" {
					homeBin = dir
				}
				if gopath != "" && strings.HasPrefix(dir, gopath) && goHomeBin == "" {
					goHomeBin = dir
				}
			}
			if bin == "" {
				bin = dir
			}
		}
	}
	if goHomeBin != "" {
		return goHomeBin
	}
	if homeBin != "" {
		return homeBin
	}
	return bin
}

// read the body or an error message
func readBody(in io.Reader) string {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return fmt.Sprintf("Error reading body: %v", err.Error())
	}
	return string(data)
}

// Get an asset URL and name
func getAsset(project string, matchName *regexp.Regexp) (string, string) {
	url := "https://api.github.com/repos/" + project + "/releases/latest"
	log.Printf("Fetching asset info for %q from %q", project, url)
	user, pass := os.Getenv("GITHUB_USER"), os.Getenv("GITHUB_TOKEN")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to make http request %q: %v", url, err)
	}
	if user != "" && pass != "" {
		log.Printf("Fetching using GITHUB_USER and GITHUB_TOKEN")
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch release info %q: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: %s", readBody(resp.Body))
		log.Fatalf("Bad status %d when fetching %q release info: %s", resp.StatusCode, url, resp.Status)
	}
	var release Release
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		log.Fatalf("Failed to decode release info: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		log.Fatalf("Failed to close body: %v", err)
	}

	for _, asset := range release.Assets {
		//log.Printf("Finding %s", asset.Name)
		if matchName.MatchString(asset.Name) && isOurOsArch(asset.Name) {
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	log.Fatalf("Didn't find asset in info")
	return "", ""
}

// Get an asset URL and name by scraping the downloads page
//
// This doesn't use the API so isn't rate limited when not using GITHUB login details
func getAssetFromReleasesPage(project string, matchName *regexp.Regexp) (assetURL string, assetName string) {
	baseURL := "https://github.com/" + project + "/releases"
	log.Printf("Fetching asset info for %q from %q", project, baseURL)
	base, err := url.Parse(baseURL)
	if err != nil {
		log.Fatalf("URL Parse failed: %v", err)
	}
	resp, err := http.Get(baseURL)
	if err != nil {
		log.Fatalf("Failed to fetch release info %q: %v", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: %s", readBody(resp.Body))
		log.Fatalf("Bad status %d when fetching %q release info: %s", resp.StatusCode, baseURL, resp.Status)
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Fatalf("Failed to parse web page: %v", err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if name := path.Base(a.Val); matchName.MatchString(name) && isOurOsArch(name) {
						if u, err := rest.URLJoin(base, a.Val); err == nil {
							if assetName == "" {
								assetName = name
								assetURL = u.String()
							}
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if assetName == "" || assetURL == "" {
		log.Fatalf("Didn't find URL in page")
	}
	return assetURL, assetName
}

// isOurOsArch returns true if s contains our OS and our Arch
func isOurOsArch(s string) bool {
	s = strings.ToLower(s)
	check := func(base string, aliases map[string][]string) bool {
		names := []string{base}
		names = append(names, aliases[base]...)
		for _, name := range names {
			if strings.Contains(s, name) {
				return true
			}
		}
		return false
	}
	return check(runtime.GOARCH, archAliases) && check(runtime.GOOS, osAliases)
}

// get a file for download
func getFile(url, fileName string) {
	log.Printf("Downloading %q from %q", fileName, url)

	out, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to open %q: %v", fileName, err)
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch asset %q: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: %s", readBody(resp.Body))
		log.Fatalf("Bad status %d when fetching %q asset: %s", resp.StatusCode, url, resp.Status)
	}

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Fatalf("Error while downloading: %v", err)
	}

	err = resp.Body.Close()
	if err != nil {
		log.Fatalf("Failed to close body: %v", err)
	}
	err = out.Close()
	if err != nil {
		log.Fatalf("Failed to close output file: %v", err)
	}

	log.Printf("Downloaded %q (%d bytes)", fileName, n)
}

// run a shell command
func run(args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run %v: %v", args, err)
	}
}

// Untars fileName from srcFile
func untar(srcFile, fileName, extractDir string) {
	f, err := os.Open(srcFile)
	if err != nil {
		log.Fatalf("Couldn't open tar: %v", err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Fatalf("Couldn't close tar: %v", err)
		}
	}()

	var in io.Reader = f

	srcExt := filepath.Ext(srcFile)
	if srcExt == ".gz" || srcExt == ".tgz" {
		gzf, err := gzip.NewReader(f)
		if err != nil {
			log.Fatalf("Couldn't open gzip: %v", err)
		}
		in = gzf
	} else if srcExt == ".bz2" {
		in = bzip2.NewReader(f)
	}

	tarReader := tar.NewReader(in)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Trouble reading tar file: %v", err)
		}
		name := header.Name
		switch header.Typeflag {
		case tar.TypeReg:
			baseName := filepath.Base(name)
			if baseName == fileName {
				outPath := filepath.Join(extractDir, fileName)
				out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
				if err != nil {
					log.Fatalf("Couldn't open output file: %v", err)
				}
				n, err := io.Copy(out, tarReader)
				if err != nil {
					log.Fatalf("Couldn't write output file: %v", err)
				}
				if err = out.Close(); err != nil {
					log.Fatalf("Couldn't close output: %v", err)
				}
				log.Printf("Wrote %s (%d bytes) as %q", fileName, n, outPath)
			}
		}
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		log.Fatalf("Syntax: %s <user/project> <name reg exp>", os.Args[0])
	}
	project, nameRe := args[0], args[1]
	if !matchProject.MatchString(project) {
		log.Fatalf("Project %q must be in form user/project", project)
	}
	matchName, err := regexp.Compile(nameRe)
	if err != nil {
		log.Fatalf("Invalid regexp for name %q: %v", nameRe, err)
	}

	var assetURL, assetName string
	if *useAPI {
		assetURL, assetName = getAsset(project, matchName)
	} else {
		assetURL, assetName = getAssetFromReleasesPage(project, matchName)
	}
	fileName := filepath.Join(os.TempDir(), assetName)
	getFile(assetURL, fileName)

	if *install {
		log.Printf("Installing %s", fileName)
		run("sudo", "dpkg", "--force-bad-version", "-i", fileName)
		log.Printf("Installed %s", fileName)
	} else if *extract != "" {
		if *bindir == "" {
			log.Fatalf("Need to set -bindir")
		}
		log.Printf("Unpacking %s from %s and installing into %s", *extract, fileName, *bindir)
		untar(fileName, *extract, *bindir+"/")
	}
}
