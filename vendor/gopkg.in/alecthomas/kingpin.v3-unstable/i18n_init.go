package kingpin

//go:generate go run ./cmd/embedi18n/main.go en-AU
//go:generate go run ./cmd/embedi18n/main.go fr

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"
	"strings"

	"github.com/nicksnyder/go-i18n/i18n"
)

type tError struct {
	msg  string
	args []interface{}
}

// TError is an error that translates itself.
//
// It has the same signature and usage as T().
func TError(msg string, args ...interface{}) error { return &tError{msg: msg, args: args} }
func (i *tError) Error() string                    { return T(i.msg, i.args...) }

// T is a translation function.
var T = initI18N()

func initI18N() i18n.TranslateFunc {
	// Initialise translations.
	i18n.ParseTranslationFileBytes("i18n/en-AU.all.json", decompressLang(i18n_en_AU))
	i18n.ParseTranslationFileBytes("i18n/fr.all.json", decompressLang(i18n_fr))

	// Detect language.
	lang := detectLang()
	t, err := i18n.Tfunc(lang, "en")
	if err != nil {
		panic(err)
	}
	return t
}

func detectLang() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		return "en"
	}
	// Remove encoding spec (eg. ".UTF-8")
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[0:idx]
	}
	// en_AU -> en-AU
	return strings.Replace(lang, "_", "-", -1)
}

func decompressLang(data []byte) []byte {
	r := bytes.NewReader(data)
	gr, err := gzip.NewReader(r)
	if err != nil {
		panic(err)
	}
	out, err := ioutil.ReadAll(gr)
	if err != nil {
		panic(err)
	}
	return out
}

// SetLanguage sets the language for Kingpin.
func SetLanguage(lang string, others ...string) error {
	t, err := i18n.Tfunc(lang, others...)
	if err != nil {
		return err
	}
	T = t
	return nil
}

// V is a convenience alias for translation function variables.
// eg. T("Something {{.Arg0}}", V{"Arg0": "moo"})
type V map[string]interface{}
