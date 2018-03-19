// Copyright 2014 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package ini_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/ini.v1"
)

type testNested struct {
	Cities      []string `delim:"|"`
	Visits      []time.Time
	Years       []int
	Numbers     []int64
	Ages        []uint
	Populations []uint64
	Coordinates []float64
	Note        string
	Unused      int `ini:"-"`
}

type TestEmbeded struct {
	GPA float64
}

type testStruct struct {
	Name         string `ini:"NAME"`
	Age          int
	Male         bool
	Money        float64
	Born         time.Time
	Time         time.Duration `ini:"Duration"`
	Others       testNested
	*TestEmbeded `ini:"grade"`
	Unused       int `ini:"-"`
	Unsigned     uint
	Omitted      bool     `ini:"omitthis,omitempty"`
	Shadows      []string `ini:",,allowshadow"`
	ShadowInts   []int    `ini:"Shadows,,allowshadow"`
}

const _CONF_DATA_STRUCT = `
NAME = Unknwon
Age = 21
Male = true
Money = 1.25
Born = 1993-10-07T20:17:05Z
Duration = 2h45m
Unsigned = 3
omitthis = true
Shadows = 1, 2
Shadows = 3, 4

[Others]
Cities = HangZhou|Boston
Visits = 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Years = 1993,1994
Numbers = 10010,10086
Ages = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
Note = Hello world!

[grade]
GPA = 2.8

[foo.bar]
Here = there
When = then
`

type unsupport struct {
	Byte byte
}

type unsupport2 struct {
	Others struct {
		Cities byte
	}
}

type Unsupport3 struct {
	Cities byte
}

type unsupport4 struct {
	*Unsupport3 `ini:"Others"`
}

type defaultValue struct {
	Name   string
	Age    int
	Male   bool
	Money  float64
	Born   time.Time
	Cities []string
}

type fooBar struct {
	Here, When string
}

const _INVALID_DATA_CONF_STRUCT = `
Name = 
Age = age
Male = 123
Money = money
Born = nil
Cities = 
`

func Test_MapToStruct(t *testing.T) {
	Convey("Map to struct", t, func() {
		Convey("Map file to struct", func() {
			ts := new(testStruct)
			So(ini.MapTo(ts, []byte(_CONF_DATA_STRUCT)), ShouldBeNil)

			So(ts.Name, ShouldEqual, "Unknwon")
			So(ts.Age, ShouldEqual, 21)
			So(ts.Male, ShouldBeTrue)
			So(ts.Money, ShouldEqual, 1.25)
			So(ts.Unsigned, ShouldEqual, 3)

			t, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			So(err, ShouldBeNil)
			So(ts.Born.String(), ShouldEqual, t.String())

			dur, err := time.ParseDuration("2h45m")
			So(err, ShouldBeNil)
			So(ts.Time.Seconds(), ShouldEqual, dur.Seconds())

			So(strings.Join(ts.Others.Cities, ","), ShouldEqual, "HangZhou,Boston")
			So(ts.Others.Visits[0].String(), ShouldEqual, t.String())
			So(fmt.Sprint(ts.Others.Years), ShouldEqual, "[1993 1994]")
			So(fmt.Sprint(ts.Others.Numbers), ShouldEqual, "[10010 10086]")
			So(fmt.Sprint(ts.Others.Ages), ShouldEqual, "[18 19]")
			So(fmt.Sprint(ts.Others.Populations), ShouldEqual, "[12345678 98765432]")
			So(fmt.Sprint(ts.Others.Coordinates), ShouldEqual, "[192.168 10.11]")
			So(ts.Others.Note, ShouldEqual, "Hello world!")
			So(ts.TestEmbeded.GPA, ShouldEqual, 2.8)
		})

		Convey("Map section to struct", func() {
			foobar := new(fooBar)
			f, err := ini.Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)

			So(f.Section("foo.bar").MapTo(foobar), ShouldBeNil)
			So(foobar.Here, ShouldEqual, "there")
			So(foobar.When, ShouldEqual, "then")
		})

		Convey("Map to non-pointer struct", func() {
			f, err := ini.Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			So(f.MapTo(testStruct{}), ShouldNotBeNil)
		})

		Convey("Map to unsupported type", func() {
			f, err := ini.Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			f.NameMapper = func(raw string) string {
				if raw == "Byte" {
					return "NAME"
				}
				return raw
			}
			So(f.MapTo(&unsupport{}), ShouldNotBeNil)
			So(f.MapTo(&unsupport2{}), ShouldNotBeNil)
			So(f.MapTo(&unsupport4{}), ShouldNotBeNil)
		})

		Convey("Map to omitempty field", func() {
			ts := new(testStruct)
			So(ini.MapTo(ts, []byte(_CONF_DATA_STRUCT)), ShouldBeNil)

			So(ts.Omitted, ShouldEqual, true)
		})

		Convey("Map with shadows", func() {
			f, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true}, []byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			ts := new(testStruct)
			So(f.MapTo(ts), ShouldBeNil)

			So(strings.Join(ts.Shadows, " "), ShouldEqual, "1 2 3 4")
			So(fmt.Sprintf("%v", ts.ShadowInts), ShouldEqual, "[1 2 3 4]")
		})

		Convey("Map from invalid data source", func() {
			So(ini.MapTo(&testStruct{}, "hi"), ShouldNotBeNil)
		})

		Convey("Map to wrong types and gain default values", func() {
			f, err := ini.Load([]byte(_INVALID_DATA_CONF_STRUCT))
			So(err, ShouldBeNil)

			t, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			So(err, ShouldBeNil)
			dv := &defaultValue{"Joe", 10, true, 1.25, t, []string{"HangZhou", "Boston"}}
			So(f.MapTo(dv), ShouldBeNil)
			So(dv.Name, ShouldEqual, "Joe")
			So(dv.Age, ShouldEqual, 10)
			So(dv.Male, ShouldBeTrue)
			So(dv.Money, ShouldEqual, 1.25)
			So(dv.Born.String(), ShouldEqual, t.String())
			So(strings.Join(dv.Cities, ","), ShouldEqual, "HangZhou,Boston")
		})
	})

	Convey("Map to struct in strict mode", t, func() {
		f, err := ini.Load([]byte(`
name=bruce
age=a30`))
		So(err, ShouldBeNil)

		type Strict struct {
			Name string `ini:"name"`
			Age  int    `ini:"age"`
		}
		s := new(Strict)

		So(f.Section("").StrictMapTo(s), ShouldNotBeNil)
	})

	Convey("Map slice in strict mode", t, func() {
		f, err := ini.Load([]byte(`
names=alice, bruce`))
		So(err, ShouldBeNil)

		type Strict struct {
			Names []string `ini:"names"`
		}
		s := new(Strict)

		So(f.Section("").StrictMapTo(s), ShouldBeNil)
		So(fmt.Sprint(s.Names), ShouldEqual, "[alice bruce]")
	})
}

func Test_ReflectFromStruct(t *testing.T) {
	Convey("Reflect from struct", t, func() {
		type Embeded struct {
			Dates       []time.Time `delim:"|" comment:"Time data"`
			Places      []string
			Years       []int
			Numbers     []int64
			Ages        []uint
			Populations []uint64
			Coordinates []float64
			None        []int
		}
		type Author struct {
			Name      string `ini:"NAME"`
			Male      bool
			Age       int `comment:"Author's age"`
			Height    uint
			GPA       float64
			Date      time.Time
			NeverMind string `ini:"-"`
			*Embeded  `ini:"infos" comment:"Embeded section"`
		}

		t, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
		So(err, ShouldBeNil)
		a := &Author{"Unknwon", true, 21, 100, 2.8, t, "",
			&Embeded{
				[]time.Time{t, t},
				[]string{"HangZhou", "Boston"},
				[]int{1993, 1994},
				[]int64{10010, 10086},
				[]uint{18, 19},
				[]uint64{12345678, 98765432},
				[]float64{192.168, 10.11},
				[]int{},
			}}
		cfg := ini.Empty()
		So(ini.ReflectFrom(cfg, a), ShouldBeNil)

		var buf bytes.Buffer
		_, err = cfg.WriteTo(&buf)
		So(err, ShouldBeNil)
		So(buf.String(), ShouldEqual, `NAME   = Unknwon
Male   = true
; Author's age
Age    = 21
Height = 100
GPA    = 2.8
Date   = 1993-10-07T20:17:05Z

; Embeded section
[infos]
; Time data
Dates       = 1993-10-07T20:17:05Z|1993-10-07T20:17:05Z
Places      = HangZhou,Boston
Years       = 1993,1994
Numbers     = 10010,10086
Ages        = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
None        = 

`)

		Convey("Reflect from non-point struct", func() {
			So(ini.ReflectFrom(cfg, Author{}), ShouldNotBeNil)
		})

		Convey("Reflect from struct with omitempty", func() {
			cfg := ini.Empty()
			type SpecialStruct struct {
				FirstName  string    `ini:"first_name"`
				LastName   string    `ini:"last_name"`
				JustOmitMe string    `ini:"omitempty"`
				LastLogin  time.Time `ini:"last_login,omitempty"`
				LastLogin2 time.Time `ini:",omitempty"`
				NotEmpty   int       `ini:"omitempty"`
			}

			So(ini.ReflectFrom(cfg, &SpecialStruct{FirstName: "John", LastName: "Doe", NotEmpty: 9}), ShouldBeNil)

			var buf bytes.Buffer
			_, err = cfg.WriteTo(&buf)
			So(buf.String(), ShouldEqual, `first_name = John
last_name  = Doe
omitempty  = 9

`)
		})
	})
}

type testMapper struct {
	PackageName string
}

func Test_NameGetter(t *testing.T) {
	Convey("Test name mappers", t, func() {
		So(ini.MapToWithMapper(&testMapper{}, ini.TitleUnderscore, []byte("packag_name=ini")), ShouldBeNil)

		cfg, err := ini.Load([]byte("PACKAGE_NAME=ini"))
		So(err, ShouldBeNil)
		So(cfg, ShouldNotBeNil)

		cfg.NameMapper = ini.AllCapsUnderscore
		tg := new(testMapper)
		So(cfg.MapTo(tg), ShouldBeNil)
		So(tg.PackageName, ShouldEqual, "ini")
	})
}

type testDurationStruct struct {
	Duration time.Duration `ini:"Duration"`
}

func Test_Duration(t *testing.T) {
	Convey("Duration less than 16m50s", t, func() {
		ds := new(testDurationStruct)
		So(ini.MapTo(ds, []byte("Duration=16m49s")), ShouldBeNil)

		dur, err := time.ParseDuration("16m49s")
		So(err, ShouldBeNil)
		So(ds.Duration.Seconds(), ShouldEqual, dur.Seconds())
	})
}
