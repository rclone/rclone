//go:build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var AssetDir http.FileSystem = http.Dir("./tzdata")
	err := vfsgen.Generate(AssetDir, vfsgen.Options{
		PackageName:  "sharefile",
		BuildTags:    "!dev",
		VariableName: "tzdata",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
