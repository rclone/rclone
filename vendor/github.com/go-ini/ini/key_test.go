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

func TestKey_AddShadow(t *testing.T) {
	Convey("Add shadow to a key", t, func() {
		f, err := ini.ShadowLoad([]byte(`
[notes]
-: note1`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(k.AddShadow("ini.v1"), ShouldBeNil)
		So(k.ValueWithShadows(), ShouldResemble, []string{"ini", "ini.v1"})

		Convey("Add shadow to boolean key", func() {
			k, err := f.Section("").NewBooleanKey("published")
			So(err, ShouldBeNil)
			So(k, ShouldNotBeNil)
			So(k.AddShadow("beta"), ShouldNotBeNil)
		})

		Convey("Add shadow to auto-increment key", func() {
			So(f.Section("notes").Key("#1").AddShadow("beta"), ShouldNotBeNil)
		})
	})

	Convey("Shadow is not allowed", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(k.AddShadow("ini.v1"), ShouldNotBeNil)
	})
}

// Helpers for slice tests.
func float64sEqual(values []float64, expected ...float64) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i], ShouldEqual, v)
	}
}

func intsEqual(values []int, expected ...int) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i], ShouldEqual, v)
	}
}

func int64sEqual(values []int64, expected ...int64) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i], ShouldEqual, v)
	}
}

func uintsEqual(values []uint, expected ...uint) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i], ShouldEqual, v)
	}
}

func uint64sEqual(values []uint64, expected ...uint64) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i], ShouldEqual, v)
	}
}

func timesEqual(values []time.Time, expected ...time.Time) {
	So(values, ShouldHaveLength, len(expected))
	for i, v := range expected {
		So(values[i].String(), ShouldEqual, v.String())
	}
}

func TestKey_Helpers(t *testing.T) {
	Convey("Getting and setting values", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		Convey("Get string representation", func() {
			sec := f.Section("")
			So(sec, ShouldNotBeNil)
			So(sec.Key("NAME").Value(), ShouldEqual, "ini")
			So(sec.Key("NAME").String(), ShouldEqual, "ini")
			So(sec.Key("NAME").Validate(func(in string) string {
				return in
			}), ShouldEqual, "ini")
			So(sec.Key("NAME").Comment, ShouldEqual, "; Package name")
			So(sec.Key("IMPORT_PATH").String(), ShouldEqual, "gopkg.in/ini.v1")

			Convey("With ValueMapper", func() {
				f.ValueMapper = func(in string) string {
					if in == "gopkg.in/%(NAME)s.%(VERSION)s" {
						return "github.com/go-ini/ini"
					}
					return in
				}
				So(sec.Key("IMPORT_PATH").String(), ShouldEqual, "github.com/go-ini/ini")
			})
		})

		Convey("Get values in non-default section", func() {
			sec := f.Section("author")
			So(sec, ShouldNotBeNil)
			So(sec.Key("NAME").String(), ShouldEqual, "Unknwon")
			So(sec.Key("GITHUB").String(), ShouldEqual, "https://github.com/Unknwon")

			sec = f.Section("package")
			So(sec, ShouldNotBeNil)
			So(sec.Key("CLONE_URL").String(), ShouldEqual, "https://gopkg.in/ini.v1")
		})

		Convey("Get auto-increment key names", func() {
			keys := f.Section("features").Keys()
			for i, k := range keys {
				So(k.Name(), ShouldEqual, fmt.Sprintf("#%d", i+1))
			}
		})

		Convey("Get parent-keys that are available to the child section", func() {
			parentKeys := f.Section("package.sub").ParentKeys()
			for _, k := range parentKeys {
				So(k.Name(), ShouldEqual, "CLONE_URL")
			}
		})

		Convey("Get overwrite value", func() {
			So(f.Section("author").Key("E-MAIL").String(), ShouldEqual, "u@gogs.io")
		})

		Convey("Get sections", func() {
			sections := f.Sections()
			for i, name := range []string{ini.DEFAULT_SECTION, "author", "package", "package.sub", "features", "types", "array", "note", "comments", "string escapes", "advance"} {
				So(sections[i].Name(), ShouldEqual, name)
			}
		})

		Convey("Get parent section value", func() {
			So(f.Section("package.sub").Key("CLONE_URL").String(), ShouldEqual, "https://gopkg.in/ini.v1")
			So(f.Section("package.fake.sub").Key("CLONE_URL").String(), ShouldEqual, "https://gopkg.in/ini.v1")
		})

		Convey("Get multiple line value", func() {
			So(f.Section("author").Key("BIO").String(), ShouldEqual, "Gopher.\nCoding addict.\nGood man.\n")
		})

		Convey("Get values with type", func() {
			sec := f.Section("types")
			v1, err := sec.Key("BOOL").Bool()
			So(err, ShouldBeNil)
			So(v1, ShouldBeTrue)

			v1, err = sec.Key("BOOL_FALSE").Bool()
			So(err, ShouldBeNil)
			So(v1, ShouldBeFalse)

			v2, err := sec.Key("FLOAT64").Float64()
			So(err, ShouldBeNil)
			So(v2, ShouldEqual, 1.25)

			v3, err := sec.Key("INT").Int()
			So(err, ShouldBeNil)
			So(v3, ShouldEqual, 10)

			v4, err := sec.Key("INT").Int64()
			So(err, ShouldBeNil)
			So(v4, ShouldEqual, 10)

			v5, err := sec.Key("UINT").Uint()
			So(err, ShouldBeNil)
			So(v5, ShouldEqual, 3)

			v6, err := sec.Key("UINT").Uint64()
			So(err, ShouldBeNil)
			So(v6, ShouldEqual, 3)

			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			v7, err := sec.Key("TIME").Time()
			So(err, ShouldBeNil)
			So(v7.String(), ShouldEqual, t.String())

			Convey("Must get values with type", func() {
				So(sec.Key("STRING").MustString("404"), ShouldEqual, "str")
				So(sec.Key("BOOL").MustBool(), ShouldBeTrue)
				So(sec.Key("FLOAT64").MustFloat64(), ShouldEqual, 1.25)
				So(sec.Key("INT").MustInt(), ShouldEqual, 10)
				So(sec.Key("INT").MustInt64(), ShouldEqual, 10)
				So(sec.Key("UINT").MustUint(), ShouldEqual, 3)
				So(sec.Key("UINT").MustUint64(), ShouldEqual, 3)
				So(sec.Key("TIME").MustTime().String(), ShouldEqual, t.String())

				dur, err := time.ParseDuration("2h45m")
				So(err, ShouldBeNil)
				So(sec.Key("DURATION").MustDuration().Seconds(), ShouldEqual, dur.Seconds())

				Convey("Must get values with default value", func() {
					So(sec.Key("STRING_404").MustString("404"), ShouldEqual, "404")
					So(sec.Key("BOOL_404").MustBool(true), ShouldBeTrue)
					So(sec.Key("FLOAT64_404").MustFloat64(2.5), ShouldEqual, 2.5)
					So(sec.Key("INT_404").MustInt(15), ShouldEqual, 15)
					So(sec.Key("INT64_404").MustInt64(15), ShouldEqual, 15)
					So(sec.Key("UINT_404").MustUint(6), ShouldEqual, 6)
					So(sec.Key("UINT64_404").MustUint64(6), ShouldEqual, 6)

					t, err := time.Parse(time.RFC3339, "2014-01-01T20:17:05Z")
					So(err, ShouldBeNil)
					So(sec.Key("TIME_404").MustTime(t).String(), ShouldEqual, t.String())

					So(sec.Key("DURATION_404").MustDuration(dur).Seconds(), ShouldEqual, dur.Seconds())

					Convey("Must should set default as key value", func() {
						So(sec.Key("STRING_404").String(), ShouldEqual, "404")
						So(sec.Key("BOOL_404").String(), ShouldEqual, "true")
						So(sec.Key("FLOAT64_404").String(), ShouldEqual, "2.5")
						So(sec.Key("INT_404").String(), ShouldEqual, "15")
						So(sec.Key("INT64_404").String(), ShouldEqual, "15")
						So(sec.Key("UINT_404").String(), ShouldEqual, "6")
						So(sec.Key("UINT64_404").String(), ShouldEqual, "6")
						So(sec.Key("TIME_404").String(), ShouldEqual, "2014-01-01T20:17:05Z")
						So(sec.Key("DURATION_404").String(), ShouldEqual, "2h45m0s")
					})
				})
			})
		})

		Convey("Get value with candidates", func() {
			sec := f.Section("types")
			So(sec.Key("STRING").In("", []string{"str", "arr", "types"}), ShouldEqual, "str")
			So(sec.Key("FLOAT64").InFloat64(0, []float64{1.25, 2.5, 3.75}), ShouldEqual, 1.25)
			So(sec.Key("INT").InInt(0, []int{10, 20, 30}), ShouldEqual, 10)
			So(sec.Key("INT").InInt64(0, []int64{10, 20, 30}), ShouldEqual, 10)
			So(sec.Key("UINT").InUint(0, []uint{3, 6, 9}), ShouldEqual, 3)
			So(sec.Key("UINT").InUint64(0, []uint64{3, 6, 9}), ShouldEqual, 3)

			zt, err := time.Parse(time.RFC3339, "0001-01-01T01:00:00Z")
			So(err, ShouldBeNil)
			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			So(sec.Key("TIME").InTime(zt, []time.Time{t, time.Now(), time.Now().Add(1 * time.Second)}).String(), ShouldEqual, t.String())

			Convey("Get value with candidates and default value", func() {
				So(sec.Key("STRING_404").In("str", []string{"str", "arr", "types"}), ShouldEqual, "str")
				So(sec.Key("FLOAT64_404").InFloat64(1.25, []float64{1.25, 2.5, 3.75}), ShouldEqual, 1.25)
				So(sec.Key("INT_404").InInt(10, []int{10, 20, 30}), ShouldEqual, 10)
				So(sec.Key("INT64_404").InInt64(10, []int64{10, 20, 30}), ShouldEqual, 10)
				So(sec.Key("UINT_404").InUint(3, []uint{3, 6, 9}), ShouldEqual, 3)
				So(sec.Key("UINT_404").InUint64(3, []uint64{3, 6, 9}), ShouldEqual, 3)
				So(sec.Key("TIME_404").InTime(t, []time.Time{time.Now(), time.Now(), time.Now().Add(1 * time.Second)}).String(), ShouldEqual, t.String())
			})
		})

		Convey("Get values in range", func() {
			sec := f.Section("types")
			So(sec.Key("FLOAT64").RangeFloat64(0, 1, 2), ShouldEqual, 1.25)
			So(sec.Key("INT").RangeInt(0, 10, 20), ShouldEqual, 10)
			So(sec.Key("INT").RangeInt64(0, 10, 20), ShouldEqual, 10)

			minT, err := time.Parse(time.RFC3339, "0001-01-01T01:00:00Z")
			So(err, ShouldBeNil)
			midT, err := time.Parse(time.RFC3339, "2013-01-01T01:00:00Z")
			So(err, ShouldBeNil)
			maxT, err := time.Parse(time.RFC3339, "9999-01-01T01:00:00Z")
			So(err, ShouldBeNil)
			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			So(sec.Key("TIME").RangeTime(t, minT, maxT).String(), ShouldEqual, t.String())

			Convey("Get value in range with default value", func() {
				So(sec.Key("FLOAT64").RangeFloat64(5, 0, 1), ShouldEqual, 5)
				So(sec.Key("INT").RangeInt(7, 0, 5), ShouldEqual, 7)
				So(sec.Key("INT").RangeInt64(7, 0, 5), ShouldEqual, 7)
				So(sec.Key("TIME").RangeTime(t, minT, midT).String(), ShouldEqual, t.String())
			})
		})

		Convey("Get values into slice", func() {
			sec := f.Section("array")
			So(strings.Join(sec.Key("STRINGS").Strings(","), ","), ShouldEqual, "en,zh,de")
			So(len(sec.Key("STRINGS_404").Strings(",")), ShouldEqual, 0)

			vals1 := sec.Key("FLOAT64S").Float64s(",")
			float64sEqual(vals1, 1.1, 2.2, 3.3)

			vals2 := sec.Key("INTS").Ints(",")
			intsEqual(vals2, 1, 2, 3)

			vals3 := sec.Key("INTS").Int64s(",")
			int64sEqual(vals3, 1, 2, 3)

			vals4 := sec.Key("UINTS").Uints(",")
			uintsEqual(vals4, 1, 2, 3)

			vals5 := sec.Key("UINTS").Uint64s(",")
			uint64sEqual(vals5, 1, 2, 3)

			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			vals6 := sec.Key("TIMES").Times(",")
			timesEqual(vals6, t, t, t)
		})

		Convey("Test string slice escapes", func() {
			sec := f.Section("string escapes")
			So(sec.Key("key1").Strings(","), ShouldResemble, []string{"value1", "value2", "value3"})
			So(sec.Key("key2").Strings(","), ShouldResemble, []string{"value1, value2"})
			So(sec.Key("key3").Strings(","), ShouldResemble, []string{`val\ue1`, "value2"})
			So(sec.Key("key4").Strings(","), ShouldResemble, []string{`value1\`, `value\\2`})
			So(sec.Key("key5").Strings(",,"), ShouldResemble, []string{"value1,, value2"})
			So(sec.Key("key6").Strings(" "), ShouldResemble, []string{"aaa", "bbb and space", "ccc"})
		})

		Convey("Get valid values into slice", func() {
			sec := f.Section("array")
			vals1 := sec.Key("FLOAT64S").ValidFloat64s(",")
			float64sEqual(vals1, 1.1, 2.2, 3.3)

			vals2 := sec.Key("INTS").ValidInts(",")
			intsEqual(vals2, 1, 2, 3)

			vals3 := sec.Key("INTS").ValidInt64s(",")
			int64sEqual(vals3, 1, 2, 3)

			vals4 := sec.Key("UINTS").ValidUints(",")
			uintsEqual(vals4, 1, 2, 3)

			vals5 := sec.Key("UINTS").ValidUint64s(",")
			uint64sEqual(vals5, 1, 2, 3)

			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			vals6 := sec.Key("TIMES").ValidTimes(",")
			timesEqual(vals6, t, t, t)
		})

		Convey("Get values one type into slice of another type", func() {
			sec := f.Section("array")
			vals1 := sec.Key("STRINGS").ValidFloat64s(",")
			So(vals1, ShouldBeEmpty)

			vals2 := sec.Key("STRINGS").ValidInts(",")
			So(vals2, ShouldBeEmpty)

			vals3 := sec.Key("STRINGS").ValidInt64s(",")
			So(vals3, ShouldBeEmpty)

			vals4 := sec.Key("STRINGS").ValidUints(",")
			So(vals4, ShouldBeEmpty)

			vals5 := sec.Key("STRINGS").ValidUint64s(",")
			So(vals5, ShouldBeEmpty)

			vals6 := sec.Key("STRINGS").ValidTimes(",")
			So(vals6, ShouldBeEmpty)
		})

		Convey("Get valid values into slice without errors", func() {
			sec := f.Section("array")
			vals1, err := sec.Key("FLOAT64S").StrictFloat64s(",")
			So(err, ShouldBeNil)
			float64sEqual(vals1, 1.1, 2.2, 3.3)

			vals2, err := sec.Key("INTS").StrictInts(",")
			So(err, ShouldBeNil)
			intsEqual(vals2, 1, 2, 3)

			vals3, err := sec.Key("INTS").StrictInt64s(",")
			So(err, ShouldBeNil)
			int64sEqual(vals3, 1, 2, 3)

			vals4, err := sec.Key("UINTS").StrictUints(",")
			So(err, ShouldBeNil)
			uintsEqual(vals4, 1, 2, 3)

			vals5, err := sec.Key("UINTS").StrictUint64s(",")
			So(err, ShouldBeNil)
			uint64sEqual(vals5, 1, 2, 3)

			t, err := time.Parse(time.RFC3339, "2015-01-01T20:17:05Z")
			So(err, ShouldBeNil)
			vals6, err := sec.Key("TIMES").StrictTimes(",")
			So(err, ShouldBeNil)
			timesEqual(vals6, t, t, t)
		})

		Convey("Get invalid values into slice", func() {
			sec := f.Section("array")
			vals1, err := sec.Key("STRINGS").StrictFloat64s(",")
			So(vals1, ShouldBeEmpty)
			So(err, ShouldNotBeNil)

			vals2, err := sec.Key("STRINGS").StrictInts(",")
			So(vals2, ShouldBeEmpty)
			So(err, ShouldNotBeNil)

			vals3, err := sec.Key("STRINGS").StrictInt64s(",")
			So(vals3, ShouldBeEmpty)
			So(err, ShouldNotBeNil)

			vals4, err := sec.Key("STRINGS").StrictUints(",")
			So(vals4, ShouldBeEmpty)
			So(err, ShouldNotBeNil)

			vals5, err := sec.Key("STRINGS").StrictUint64s(",")
			So(vals5, ShouldBeEmpty)
			So(err, ShouldNotBeNil)

			vals6, err := sec.Key("STRINGS").StrictTimes(",")
			So(vals6, ShouldBeEmpty)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestKey_StringsWithShadows(t *testing.T) {
	Convey("Get strings of shadows of a key", t, func() {
		f, err := ini.ShadowLoad([]byte(""))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NUMS", "1,2")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("NUMS", "4,5,6")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(k.StringsWithShadows(","), ShouldResemble, []string{"1", "2", "4", "5", "6"})
	})
}

func TestKey_SetValue(t *testing.T) {
	Convey("Set value of key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		So(k.Value(), ShouldEqual, "ini")

		k.SetValue("ini.v1")
		So(k.Value(), ShouldEqual, "ini.v1")
	})
}

func TestKey_NestedValues(t *testing.T) {
	Convey("Read and write nested values", t, func() {
		f, err := ini.LoadSources(ini.LoadOptions{
			AllowNestedValues: true,
		}, []byte(`
aws_access_key_id = foo
aws_secret_access_key = bar
region = us-west-2
s3 =
  max_concurrent_requests=10
  max_queue_size=1000`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		So(f.Section("").Key("s3").NestedValues(), ShouldResemble, []string{"max_concurrent_requests=10", "max_queue_size=1000"})

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		So(err, ShouldBeNil)
		So(buf.String(), ShouldEqual, `aws_access_key_id     = foo
aws_secret_access_key = bar
region                = us-west-2
s3                    = 
  max_concurrent_requests=10
  max_queue_size=1000

`)
	})
}

func TestRecursiveValues(t *testing.T) {
	Convey("Recursive values should not reflect on same key", t, func() {
		f, err := ini.Load([]byte(`
NAME = ini
[package]
NAME = %(NAME)s`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)
		So(f.Section("package").Key("NAME").String(), ShouldEqual, "ini")
	})
}
