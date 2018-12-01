package plog

import (
	"encoding/json"
	"fmt"

	"github.com/gobuffalo/logger"
	"github.com/sirupsen/logrus"
)

var Logger = logger.New(logger.ErrorLevel)

func Debug(t interface{}, m string, args ...interface{}) {
	if len(args)%2 == 1 {
		args = append(args, "")
	}
	f := logrus.Fields{}
	for i := 0; i < len(args); i += 2 {
		k := fmt.Sprint(args[i])
		v := args[i+1]
		if s, ok := v.(fmt.Stringer); ok {
			f[k] = s.String()
			continue
		}
		if s, ok := v.(string); ok {
			f[k] = s
			continue
		}
		if b, err := json.Marshal(v); err == nil {
			f[k] = string(b)
			continue
		}
		f[k] = v
	}
	e := Logger.WithFields(f)
	if s, ok := t.(string); ok {
		e.Debugf("%s#%s", s, m)
		return
	}
	e.Debugf("%T#%s", t, m)
}
