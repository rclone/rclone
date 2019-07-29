package configstruct_test

import (
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conf struct {
	A string
	B string
}

type conf2 struct {
	PotatoPie      string `config:"spud_pie"`
	BeanStew       bool
	RaisinRoll     int
	SausageOnStick int64
	ForbiddenFruit uint
	CookingTime    fs.Duration
	TotalWeight    fs.SizeSuffix
}

func TestItemsError(t *testing.T) {
	_, err := configstruct.Items(nil)
	assert.EqualError(t, err, "argument must be a pointer")
	_, err = configstruct.Items(new(int))
	assert.EqualError(t, err, "argument must be a pointer to a struct")
}

func TestItems(t *testing.T) {
	in := &conf2{
		PotatoPie:      "yum",
		BeanStew:       true,
		RaisinRoll:     42,
		SausageOnStick: 101,
		ForbiddenFruit: 6,
		CookingTime:    fs.Duration(42 * time.Second),
		TotalWeight:    fs.SizeSuffix(17 << 20),
	}
	got, err := configstruct.Items(in)
	require.NoError(t, err)
	want := []configstruct.Item{
		{Name: "spud_pie", Field: "PotatoPie", Num: 0, Value: string("yum")},
		{Name: "bean_stew", Field: "BeanStew", Num: 1, Value: true},
		{Name: "raisin_roll", Field: "RaisinRoll", Num: 2, Value: int(42)},
		{Name: "sausage_on_stick", Field: "SausageOnStick", Num: 3, Value: int64(101)},
		{Name: "forbidden_fruit", Field: "ForbiddenFruit", Num: 4, Value: uint(6)},
		{Name: "cooking_time", Field: "CookingTime", Num: 5, Value: fs.Duration(42 * time.Second)},
		{Name: "total_weight", Field: "TotalWeight", Num: 6, Value: fs.SizeSuffix(17 << 20)},
	}
	assert.Equal(t, want, got)
}

func TestSetBasics(t *testing.T) {
	c := &conf{A: "one", B: "two"}
	err := configstruct.Set(configMap{}, c)
	require.NoError(t, err)
	assert.Equal(t, &conf{A: "one", B: "two"}, c)
}

// a simple configmap.Getter for testing
type configMap map[string]string

// Get the value
func (c configMap) Get(key string) (value string, ok bool) {
	value, ok = c[key]
	return value, ok
}

func TestSetMore(t *testing.T) {
	c := &conf{A: "one", B: "two"}
	m := configMap{
		"a": "ONE",
	}
	err := configstruct.Set(m, c)
	require.NoError(t, err)
	assert.Equal(t, &conf{A: "ONE", B: "two"}, c)
}

func TestSetFull(t *testing.T) {
	in := &conf2{
		PotatoPie:      "yum",
		BeanStew:       true,
		RaisinRoll:     42,
		SausageOnStick: 101,
		ForbiddenFruit: 6,
		CookingTime:    fs.Duration(42 * time.Second),
		TotalWeight:    fs.SizeSuffix(17 << 20),
	}
	m := configMap{
		"spud_pie":         "YUM",
		"bean_stew":        "FALSE",
		"raisin_roll":      "43 ",
		"sausage_on_stick": " 102 ",
		"forbidden_fruit":  "0x7",
		"cooking_time":     "43s",
		"total_weight":     "18M",
	}
	want := &conf2{
		PotatoPie:      "YUM",
		BeanStew:       false,
		RaisinRoll:     43,
		SausageOnStick: 102,
		ForbiddenFruit: 7,
		CookingTime:    fs.Duration(43 * time.Second),
		TotalWeight:    fs.SizeSuffix(18 << 20),
	}
	err := configstruct.Set(m, in)
	require.NoError(t, err)
	assert.Equal(t, want, in)
}
