package flect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gobuffalo/envy"
)

func init() {
	loadCustomData("inflections.json", "INFLECT_PATH", "could not read inflection file", LoadInflections)
	loadCustomData("acronyms.json", "ACRONYMS_PATH", "could not read acronyms file", LoadAcronyms)
}

//CustomDataParser are functions that parse data like acronyms or
//plurals in the shape of a io.Reader it receives.
type CustomDataParser func(io.Reader) error

func loadCustomData(defaultFile, env, readErrorMessage string, parser CustomDataParser) {
	pwd, _ := os.Getwd()
	path := envy.Get(env, filepath.Join(pwd, defaultFile))

	if _, err := os.Stat(path); err != nil {
		return
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("%s %s (%s)\n", readErrorMessage, path, err)
		return
	}

	if err = parser(bytes.NewReader(b)); err != nil {
		fmt.Println(err)
	}
}

//LoadAcronyms loads rules from io.Reader param
func LoadAcronyms(r io.Reader) error {
	m := []string{}
	err := json.NewDecoder(r).Decode(&m)

	if err != nil {
		return fmt.Errorf("could not decode acronyms JSON from reader: %s", err)
	}

	acronymsMoot.Lock()
	defer acronymsMoot.Unlock()

	for _, acronym := range m {
		baseAcronyms[acronym] = true
	}

	return nil
}

//LoadInflections loads rules from io.Reader param
func LoadInflections(r io.Reader) error {
	m := map[string]string{}

	err := json.NewDecoder(r).Decode(&m)
	if err != nil {
		return fmt.Errorf("could not decode inflection JSON from reader: %s", err)
	}

	pluralMoot.Lock()
	defer pluralMoot.Unlock()
	singularMoot.Lock()
	defer singularMoot.Unlock()

	for s, p := range m {
		singleToPlural[s] = p
		pluralToSingle[p] = s
	}

	return nil
}
