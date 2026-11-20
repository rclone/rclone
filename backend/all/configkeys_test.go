package all_test

import (
	"go/ast"
	"go/constant"
	"go/types"
	"reflect"
	"runtime"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// dynamicConfigKeys lists the config keys which are computed at
// runtime so can't be checked statically, by package and expression.
//
// These are still checked at runtime by the config file setter.
var dynamicConfigKeys = map[string]map[string]bool{
	"github.com/rclone/rclone/backend/sftp": {
		`fmt.Sprintf("%vsum_command", hashType)`: true,
	},
}

// backendPackage returns the import path of the package which
// registered ri.
func backendPackage(ri *fs.RegInfo) string {
	name := runtime.FuncForPC(reflect.ValueOf(ri.NewFs).Pointer()).Name()
	// name is like "github.com/rclone/rclone/backend/drive.NewFs" or
	// "github.com/rclone/rclone/backend/drive.init.func1"
	slash := strings.LastIndex(name, "/")
	if dot := strings.Index(name[slash:], "."); dot >= 0 {
		name = name[:slash+dot]
	}
	return name
}

// isConfigmapSetter returns true if t is a configmap type which
// saves config (but not configmap.Simple which is just a map).
func isConfigmapSetter(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return false
	}
	return n.Obj().Pkg().Path() == "github.com/rclone/rclone/fs/config/configmap" && n.Obj().Name() != "Simple"
}

// configKeyArg returns the argument holding the config key if call
// saves config, or nil if it doesn't.
func configKeyArg(info *types.Info, call *ast.CallExpr) (keyArg ast.Expr, isPutToken bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	switch sel.Sel.Name {
	case "Set":
		if tv, ok := info.Types[sel.X]; ok && len(call.Args) == 2 && isConfigmapSetter(tv.Type) {
			return call.Args[0], false
		}
	case "PutToken":
		if obj := info.Uses[sel.Sel]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "github.com/rclone/rclone/lib/oauthutil" {
			return nil, true
		}
	case "ConfigFileSet", "FileSetValue", "SetValueAndSave":
		if len(call.Args) == 3 {
			return call.Args[1], false
		}
	}
	return nil, false
}

// TestConfigKeysHaveOptions checks that every config key a backend
// saves is one of its config options.
//
// Keys with no option aren't redacted by "rclone config redacted" and
// can't be set with flags or environment variables.
func TestConfigKeysHaveOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode as it type checks all the backends")
	}

	// Find the option names for each backend package
	options := map[string]map[string]bool{}
	for _, ri := range fs.Registry {
		pkg := backendPackage(ri)
		if options[pkg] == nil {
			options[pkg] = map[string]bool{}
		}
		for _, opt := range ri.Options {
			options[pkg][opt.Name] = true
		}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  "../..",
	}
	pkgs, err := packages.Load(cfg, "./backend/...")
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)

	checked := 0
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			t.Errorf("%s: %v", pkg.PkgPath, e)
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				keyArg, isPutToken := configKeyArg(pkg.TypesInfo, call)
				var key string
				switch {
				case isPutToken:
					key = "token"
				case keyArg == nil:
					return true
				default:
					tv := pkg.TypesInfo.Types[keyArg]
					if tv.Value == nil || tv.Value.Kind() != constant.String {
						expr := types.ExprString(keyArg)
						assert.True(t, dynamicConfigKeys[pkg.PkgPath][expr], "%v: config key %s isn't a constant - add it to dynamicConfigKeys if this is intended", pkg.Fset.Position(call.Pos()), expr)
						return true
					}
					key = constant.StringVal(tv.Value)
				}
				checked++
				assert.True(t, options[pkg.PkgPath][key], "%v: config key %q has no config option in %s", pkg.Fset.Position(call.Pos()), key, pkg.PkgPath)
				return true
			})
		}
	}
	// Make sure the search is finding things
	assert.Greater(t, checked, 50)
}
