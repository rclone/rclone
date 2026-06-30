package mountlib

import (
	"reflect"
	"sync"

	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
)

var (
	vfsOptionsOnce   sync.Once
	vfsOptionsMap    map[string]bool
	mountOptionsOnce sync.Once
	mountOptionsMap  map[string]bool
)

func initVfsOptions() {
	vfsOptionsOnce.Do(func() {
		vfsOptionsMap = make(map[string]bool, len(vfscommon.OptionsInfo))
		for _, opt := range vfscommon.OptionsInfo {
			vfsOptionsMap[opt.Name] = true
		}
	})
}

func initMountOptions() {
	mountOptionsOnce.Do(func() {
		mountOptionsMap = make(map[string]bool, len(OptionsInfo))
		for _, opt := range OptionsInfo {
			mountOptionsMap[opt.Name] = true
		}
	})
}

// isMap returns true if v's underlying type is a map
func isMap(v any) bool {
	if v == nil {
		return false
	}
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Map
}

// parseVfsOptions parses VFS options from in (both flat and nested) and updates vfsOpt
func parseVfsOptions(in rc.Params, vfsOpt *vfscommon.Options) error {
	initVfsOptions()
	flatVfs := make(map[string]any)
	for k, v := range in {
		if vfsOptionsMap[k] {
			if isMap(v) {
				continue
			}
			flatVfs[k] = v
		}
	}
	if len(flatVfs) > 0 {
		err := configstruct.SetAny(flatVfs, vfsOpt)
		if err != nil {
			return err
		}
		for k := range flatVfs {
			delete(in, k)
		}
	}
	err := in.GetStructMissingOK("vfsOpt", vfsOpt)
	if err != nil {
		return err
	}
	delete(in, "vfsOpt")
	return nil
}

// parseMountOptions parses Mount options from in (both flat and nested) and updates mountOpt
func parseMountOptions(in rc.Params, mountOpt *Options) error {
	initMountOptions()
	flatMount := make(map[string]any)
	for k, v := range in {
		if mountOptionsMap[k] {
			if isMap(v) {
				continue
			}
			flatMount[k] = v
		}
	}
	if len(flatMount) > 0 {
		err := configstruct.SetAny(flatMount, mountOpt)
		if err != nil {
			return err
		}
		for k := range flatMount {
			delete(in, k)
		}
	}
	err := in.GetStructMissingOK("mountOpt", mountOpt)
	if err != nil {
		return err
	}
	delete(in, "mountOpt")
	return nil
}
