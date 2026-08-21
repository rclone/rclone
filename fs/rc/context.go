package rc

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/filter"
)

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

// hasOption checks if any options are present in the params
func hasOption(in Params, key string, opt any) bool {
	if _, ok := in[key]; ok {
		return true
	}
	items, err := configstruct.Items(opt)
	if err != nil {
		return false
	}
	for _, item := range items {
		if v, ok := in[item.Name]; ok && !isMap(v) {
			return true
		}
	}
	return false
}

// ParseOptions sets opt from the flat parameters in whose names match opt's
// fields, then overlays the nested block under key if present. Every key it
// consumes is deleted from in. Values in the nested block take precedence over
// flat ones. opt must be a pointer to a struct of configstruct-settable fields.
func ParseOptions(in Params, key string, opt any) error {
	items, err := configstruct.Items(opt)
	if err != nil {
		return err
	}
	flat := make(map[string]any)
	for _, item := range items {
		if v, ok := in[item.Name]; ok && !isMap(v) {
			flat[item.Name] = v
		}
	}
	if len(flat) > 0 {
		if err := configstruct.SetAny(flat, opt); err != nil {
			return err
		}
		for k := range flat {
			delete(in, k)
		}
	}
	if err := in.GetStructMissingOK(key, opt); err != nil {
		return err
	}
	delete(in, key)
	return nil
}

// CheckParamsUsed returns an error if any parameters remain in the map.
// It formats them as a list of sorted keys for determinism.
func CheckParamsUsed(in Params) error {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown parameters: %s", strings.Join(keys, ", "))
}

// AddConfig parses any config options from the parameters and returns a new context with the configuration.
func AddConfig(ctx context.Context, in Params) (context.Context, error) {
	if !hasOption(in, "_config", &fs.ConfigInfo{}) {
		return ctx, nil
	}
	ctx, ci := fs.AddConfig(ctx)
	if err := ParseOptions(in, "_config", ci); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// AddFilter parses any filter options from the parameters and returns a new context with the filter.
func AddFilter(ctx context.Context, in Params) (context.Context, error) {
	if !hasOption(in, "_filter", &filter.Opt) {
		return ctx, nil
	}
	// Copy of the current filter options
	opt := filter.GetConfig(ctx).Opt
	if err := ParseOptions(in, "_filter", &opt); err != nil {
		return ctx, err
	}
	fi, err := filter.NewFilter(&opt)
	if err != nil {
		return ctx, err
	}
	ctx = filter.ReplaceConfig(ctx, fi)
	return ctx, nil
}
