package rc

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/filter"
)

var (
	configOptionsOnce sync.Once
	configOptionsMap  map[string]bool

	filterOptionsOnce sync.Once
	filterOptionsMap  map[string]bool
)

func initConfigOptions() {
	configOptionsOnce.Do(func() {
		configOptionsMap = make(map[string]bool, len(fs.ConfigOptionsInfo))
		for _, opt := range fs.ConfigOptionsInfo {
			configOptionsMap[opt.Name] = true
		}
	})
}

func initFilterOptions() {
	filterOptionsOnce.Do(func() {
		filterOptionsMap = make(map[string]bool, len(filter.OptionsInfo))
		for _, opt := range filter.OptionsInfo {
			filterOptionsMap[opt.Name] = true
		}
	})
}

// hasConfigOption checks if any config options are present in the params
func hasConfigOption(in Params) bool {
	if _, ok := in["_config"]; ok {
		return true
	}
	initConfigOptions()
	for k := range in {
		if configOptionsMap[k] {
			return true
		}
	}
	return false
}

// hasFilterOption checks if any filter options are present in the params
func hasFilterOption(in Params) bool {
	if _, ok := in["_filter"]; ok {
		return true
	}
	initFilterOptions()
	for k := range in {
		if filterOptionsMap[k] {
			return true
		}
	}
	return false
}

// AddConfig parses any config options from the parameters and returns a new context with the configuration.
func AddConfig(ctx context.Context, in Params) (context.Context, error) {
	if !hasConfigOption(in) {
		return ctx, nil
	}
	ctx, ci := fs.AddConfig(ctx)
	err := configstruct.SetAny(in, ci)
	if err != nil {
		return ctx, err
	}
	if _, ok := in["_config"]; ok {
		err = in.GetStruct("_config", ci)
		if err != nil {
			return ctx, err
		}
		delete(in, "_config") // remove the parameter
	}
	return ctx, nil
}

// AddFilter parses any filter options from the parameters and returns a new context with the filter.
func AddFilter(ctx context.Context, in Params) (context.Context, error) {
	if !hasFilterOption(in) {
		return ctx, nil
	}
	// Copy of the current filter options
	opt := filter.GetConfig(ctx).Opt
	err := configstruct.SetAny(in, &opt)
	if err != nil {
		return ctx, err
	}
	if _, ok := in["_filter"]; ok {
		// Update the options from the parameter
		err = in.GetStruct("_filter", &opt)
		if err != nil {
			return ctx, err
		}
		delete(in, "_filter") // remove the parameter
	}
	fi, err := filter.NewFilter(&opt)
	if err != nil {
		return ctx, err
	}
	ctx = filter.ReplaceConfig(ctx, fi)
	return ctx, nil
}
