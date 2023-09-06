//go:build aix
// +build aix

package cpu

import (
	"context"
)

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}
