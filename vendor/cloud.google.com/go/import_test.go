// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloud

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestContextImport(t *testing.T) {
	t.Parallel()

	whiteList := map[string]bool{
		"storage/go17.go": true,
	}

	err := filepath.Walk(".", func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".go" || whiteList[path] {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range file.Imports {
			impPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if impPath == "context" {
				t.Errorf(`file %q import "context", want "golang.org/x/net/context"`, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
