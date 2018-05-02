// +build codegen

package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Attach opens a file by name, and unmarshal its JSON data.
// Will proceed to setup the API if not already done so.
func (a *API) Attach(filename string) {
	a.path = filepath.Dir(filename)
	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(f).Decode(a); err != nil {
		panic(fmt.Errorf("failed to decode %s, err: %v", filename, err))
	}
}

// AttachString will unmarshal a raw JSON string, and setup the
// API if not already done so.
func (a *API) AttachString(str string) {
	json.Unmarshal([]byte(str), a)

	if !a.initialized {
		a.Setup()
	}
}

// Setup initializes the API.
func (a *API) Setup() {
	a.setMetadataEndpointsKey()
	a.writeShapeNames()
	a.resolveReferences()
	a.fixStutterNames()
	a.renameExportable()
	if !a.NoRenameToplevelShapes {
		a.renameToplevelShapes()
	}

	a.renameCollidingFields()
	a.updateTopLevelShapeReferences()
	a.createInputOutputShapes()
	a.suppressEventStreams()
	a.customizationPasses()

	if !a.NoRemoveUnusedShapes {
		a.removeUnusedShapes()
	}

	if !a.NoValidataShapeMethods {
		a.addShapeValidations()
	}

	a.initialized = true
}
