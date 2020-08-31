package operations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// Creates an OptionResume that will be passed to Put/Upload
func createResumeOpt(ctx context.Context, f fs.Fs, remote string, src fs.Object) (resumeOpt *fs.OptionResume) {
	ci := fs.GetConfig(ctx)
	resumeOpt = &fs.OptionResume{ID: "", Pos: 0, Src: src, F: f, Remote: remote, CacheCleaned: false, CacheDir: config.CacheDir}
	if ci.ResumeLarger >= 0 {
		root := f.Root()
		if runtime.GOOS == "windows" {
			if root[:4] == "//?/" {
				root = root[4:]
			}
			if root[1] == ':' {
				root = strings.Replace(root, ":", "", 1)
			}
		}
		cacheName := filepath.Join(config.CacheDir, "resume", f.Name(), root, remote)
		resumeID, hashName, hashState, attemptResume := readResumeCache(ctx, f, src, cacheName)
		if attemptResume {
			fs.Debugf(f, "Existing resume cache file found: %s. A resume will now be attempted.", cacheName)
			position, resumeErr := f.Features().Resume(ctx, remote, resumeID, hashName, hashState)
			if resumeErr != nil {
				fs.Errorf(src, "Resume canceled: %v", resumeErr)
			} else if position > int64(ci.ResumeLarger) {
				(*resumeOpt).Pos = position
			}
		}
	}
	return resumeOpt
}

// readResumeCache checks to see if a resume ID has been cached for the source object.
// If it finds one it returns it along with true to signal a resume can be attempted
func readResumeCache(ctx context.Context, f fs.Fs, src fs.Object, cacheName string) (resumeID, hashName, hashState string, attemptResume bool) {
	existingCacheFile, statErr := os.Open(cacheName)
	defer func() {
		_ = existingCacheFile.Close()
	}()
	if !os.IsNotExist(statErr) {
		rawData, readErr := ioutil.ReadAll(existingCacheFile)
		if readErr == nil {
			existingFingerprint, resumeID, hashName, hashState, unmarshalErr := unmarshalResumeJSON(ctx, rawData)
			if unmarshalErr != nil {
				fs.Debugf(f, "Failed to unmarshal Resume JSON: %s. Resume will not be attempted.", unmarshalErr.Error())
			} else if existingFingerprint != "" {
				// Check if the src object has changed by comparing new Fingerprint to Fingerprint in cache file
				fingerprint := fs.Fingerprint(ctx, src, true)
				if existingFingerprint == fingerprint {
					return resumeID, hashName, hashState, true
				}
			}
		}
	}
	return "", "", "", false
}

func unmarshalResumeJSON(ctx context.Context, data []byte) (fprint, id, hashName, hashState string, err error) {
	var resumedata fs.ResumeJSON
	err = json.Unmarshal(data, &resumedata)
	if err != nil {
		return "", "", "", "", err
	}
	return resumedata.Fingerprint, resumedata.ID, resumedata.HashName, resumedata.HashState, nil
}
