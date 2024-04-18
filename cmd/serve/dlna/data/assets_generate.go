//go:generate go run assets_generate.go
// The "go:generate" directive compiles static assets by running assets_generate.go
//go:build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var AssetDir http.FileSystem = http.Dir("./static")
	err := vfsgen.Generate(AssetDir, vfsgen.Options{
		PackageName:  "data",
		BuildTags:    "!dev",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
