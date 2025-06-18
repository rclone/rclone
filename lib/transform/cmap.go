package transform

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"golang.org/x/text/encoding/charmap"
)

var (
	cmaps = map[int]*charmap.Charmap{}
	lock  sync.Mutex
)

// CharmapChoices is an enum of the character map choices.
type CharmapChoices = fs.Enum[cmapChoices]

type cmapChoices struct{}

func (cmapChoices) Choices() []string {
	choices := []string{}
	i := 0
	for _, enc := range charmap.All {
		c, ok := enc.(*charmap.Charmap)
		if !ok {
			continue
		}
		name := strings.ReplaceAll(c.String(), " ", "-")
		if name == "" {
			name = fmt.Sprintf("unknown-%d", i)
		}
		lock.Lock()
		cmaps[i] = c
		lock.Unlock()
		choices = append(choices, name)
		i++
	}
	return choices
}

func (cmapChoices) Type() string {
	return "string"
}

func charmapByID(cm fs.Enum[cmapChoices]) *charmap.Charmap {
	lock.Lock()
	c, ok := cmaps[int(cm)]
	lock.Unlock()
	if ok {
		return c
	}
	return nil
}

func encodeWithReplacement(s string, cmap *charmap.Charmap) string {
	return strings.Map(func(r rune) rune {
		b, ok := cmap.EncodeRune(r)
		if !ok {
			return '_'
		}
		return cmap.DecodeByte(b)
	}, s)
}

func toASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r <= 127 {
			return r
		}
		return -1
	}, s)
}
