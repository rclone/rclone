package syncfiles

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/operations"
)

// SyncConfigFile maintains the list of files registered for sync
const SyncConfigFile = "syncfiles.conf"

// SyncFile copies a single file and updates the config file
func SyncFile(ctx context.Context, fsrc, fdst fs.Fs, fileName string) error {
	// Get modified time of source file
	srcObj, err := fsrc.NewObject(ctx, fileName)
	if err != nil {
		return err
	}
	srcModTime := srcObj.ModTime(ctx).Unix()

	// Check if destination file exists
	dstObj, err := fdst.NewObject(ctx, fileName)
	if err == fs.ErrorObjectNotFound {
		dstObj = nil
	} else if err != nil {
		return err
	}

	doCopy := true
	if dstObj != nil {
		dstModTime := dstObj.ModTime(ctx).Unix()
		// Check if BOTH files are modified since the last sync. In
		// this case, copying can't be done
		prevModTime := getPrevModTime(fileName)
		if err == nil && srcModTime != prevModTime && dstModTime != prevModTime {
			delete(syncFilesMap, fileName)
			configUpdated = true
			return fserrors.FatalError(errors.New("Both files changed since the previous sync. Sync manually"))
		}
		switch {
		case dstModTime > srcModTime:
			// destination is newer, switch src, dst and copy
			fdst, fsrc = fsrc, fdst
			dstModTime, srcModTime = srcModTime, dstModTime
		case dstModTime < srcModTime:
			// source is newer. Copy
		default:
			// mod times are equal. Don't copy
			doCopy = false
		}
	}
	if doCopy {
		err = operations.CopyFile(ctx, fdst, fsrc, fileName, fileName)
		if err == nil && !fs.Config.DryRun {
			updateConfigFile(ctx, fsrc, fdst, fileName, srcModTime)
		}
		return err
	}
	return fs.ErrorCantCopy
}

// SyncFiles attempts to sync all the files registered in the config file
func SyncFiles(ctx context.Context) error {
	for _, m := range syncFilesMap {
		fsrc, fdst := cmd.NewFsSrcDst([]string{m.srcDir, m.dstDir})
		SyncFile(ctx, fsrc, fdst, m.fileName)
	}
	return nil
}

// Following is the line structure in the config file, tab-separated
type mapStruct struct {
	fileName string
	srcDir   string
	dstDir   string
	modTime  string
}

var syncFilesMap = make(map[string]*mapStruct)
var configUpdated = false

func getConigFilePath() string {
	configDir := filepath.Dir(config.ConfigPath)
	return filepath.Join(configDir, SyncConfigFile)
}

// LoadConfigFile loads the config file into memory
func LoadConfigFile() error {
	configFilePath := getConigFilePath()
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		return nil
	}
	file, err := os.Open(configFilePath)
	if err != nil {
		return errors.New("Failed to read config file " + configFilePath + "\n" + err.Error())
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		str := strings.Split(l, "\t")
		if len(str) != 4 {
			return errors.New("Config file " + configFilePath + " corrupted\n")
		}
		syncFilesMap[str[0]] = &mapStruct{str[0], str[1], str[2], str[3]}
	}
	configUpdated = false
	return nil
}

// SaveConfigFile saves the file to disk, if its contents have changed
func SaveConfigFile() error {
	if fs.Config.DryRun || !configUpdated || len(syncFilesMap) == 0 {
		// Nothing to save
		return nil
	}

	configFilePath := getConigFilePath()
	file, err := os.Create(configFilePath)
	if err != nil {
		return errors.New("Failed to create " + configFilePath + "\n" + err.Error())
	}
	writer := bufio.NewWriter(file)

	for _, l := range syncFilesMap {
		s := fmt.Sprintf("%s\t%s\t%s\t%s\n", l.fileName, l.srcDir, l.dstDir, l.modTime)
		_, err = writer.WriteString(s)
		if err != nil {
			file.Close()
			return errors.New("Failed to write to " + configFilePath + "\n" + err.Error())
		}
	}
	writer.Flush()
	file.Close()
	configUpdated = false
	return nil
}

func updateConfigFile(ctx context.Context, fsrc, fdst fs.Fs, fileName string, modTime int64) {
	fmap := mapStruct{
		fileName,
		mkDirName(fsrc),
		mkDirName(fdst),
		strconv.FormatInt(modTime, 10),
	}
	syncFilesMap[fileName] = &fmap
	configUpdated = true
}

func getPrevModTime(fileName string) int64 {
	// Get the file time of the previous sync, if it exists
	fmap := syncFilesMap[fileName]
	if fmap == nil {
		return 0
	}
	i, _ := strconv.ParseInt(fmap.modTime, 10, 64)
	return i
}

func mkDirName(fs fs.Fs) string {
	name := fs.Name()
	dir := fs.Root()
	if name == "local" {
		return dir
	}
	return name + ":" + dir
}
