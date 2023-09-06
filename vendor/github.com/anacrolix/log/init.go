package log

import (
	"os"
)

func init() {
	var err error
	rules, err = parseEnvRules()
	if err != nil {
		panic(err)
	}
	Default = loggerCore{
		nonZero: true,
		// This is the level if no rules apply, unless overridden in this logger, or any derived
		// loggers.
		filterLevel: Warning,
		Handlers:    []Handler{DefaultHandler},
	}.asLogger()
	Default.defaultLevel, _, err = levelFromString(os.Getenv("GO_LOG_DEFAULT_LEVEL"))
	if err != nil {
		panic(err)
	}
}
