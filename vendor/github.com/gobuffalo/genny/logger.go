package genny

import "github.com/gobuffalo/logger"

// Logger interface for a logger to be used with genny. Logrus is 100% compatible.
type Logger = logger.Logger

var DefaultLogLvl = logger.InfoLevel
