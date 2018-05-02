/*
 * opt_test.go
 *
 * Copyright 2017 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package fuse

import (
	"reflect"
	"testing"
)

func TestOptParse(t *testing.T) {
	args := []string{
		"-s",
		"--long=LONG",
		"--d=-42",
		"--d8=-8",
		"--d16=-16",
		"--d32=-32",
		"--d64=-64",
		"--u=-42",
		"--u8=-8",
		"--u16=-16",
		"--u32=-32",
		"--u64=-64",
		"--uptr=42",
		"--X=abc",
		"--O=0777",
		"--I=0xabc",
		"--S=string",
		"--V=value",
		"-o",
		"n1=v1",
		"-o",
		"n2=v2",
		"--",
		"-o",
		"n3=v3",
		"arg1",
		"arg2",
	}

	expargs := []string{
		"-o",
		"n1=v1,n2=v2",
		"-s",
		"--long=LONG",
		"--d=-42",
		"--d8=-8",
		"--d16=-16",
		"--d32=-32",
		"--d64=-64",
		"--u=-42",
		"--u8=-8",
		"--u16=-16",
		"--u32=-32",
		"--u64=-64",
		"--uptr=42",
		"--X=abc",
		"--O=0777",
		"--I=0xabc",
		"--S=string",
		"--V=value",
		"--",
		"-o",
		"n3=v3",
		"arg1",
		"arg2",
	}

	expargs2 := []string{
		"-o",
		"n3=v3",
		"arg1",
		"arg2",
	}

	var dummy bool
	outargs, err := OptParse(args, "DUMMY", &dummy)
	if nil != err {
		t.Error(err)
	}

	if !reflect.DeepEqual(expargs, outargs) {
		t.Error()
	}

	outargs, err = OptParse(args, "")
	if nil != err {
		t.Error(err)
	}

	if !reflect.DeepEqual(expargs2, outargs) {
		t.Error()
	}

	var (
		d    int
		d8   int8
		d16  int16
		d32  int32
		d64  int64
		u    uint
		u8   uint8
		u16  uint16
		u32  uint32
		u64  uint64
		uptr uintptr
	)
	outargs, err = OptParse(args,
		"--d=%d --d8=%d --d16=%d --d32=%d --d64=%d --u=%d --u8=%d --u16=%d --u32=%d --u64=%d "+
			"--uptr=%d",
		&d, &d8, &d16, &d32, &d64, &u, &u8, &u16, &u32, &u64, &uptr)
	if nil != err {
		t.Error(err)
	}

	if -42 != d || -8 != d8 || -16 != d16 || -32 != d32 || -64 != d64 {
		t.Error()
	}
	if uint(-42&0xffffffff) != u ||
		uint8(-8&0xff) != u8 ||
		uint16(-16&0xffff) != u16 ||
		uint32(-32&0xffffffff) != u32 ||
		uint64(-64&0xffffffffffffffff) != u64 ||
		uintptr(42) != uptr {
		t.Error()
	}

	d = 0
	d8 = 0
	d16 = 0
	d32 = 0
	d64 = 0
	u = 0
	u8 = 0
	u16 = 0
	u32 = 0
	u64 = 0
	uptr = 0
	outargs, err = OptParse(args,
		"--d --d8 --d16 --d32 --d64 --u --u8 --u16 --u32 --u64 "+
			"--uptr",
		&d, &d8, &d16, &d32, &d64, &u, &u8, &u16, &u32, &u64, &uptr)
	if nil != err {
		t.Error(err)
	}

	if -42 != d || -8 != d8 || -16 != d16 || -32 != d32 || -64 != d64 {
		t.Error()
	}
	if uint(-42&0xffffffff) != u ||
		uint8(-8&0xff) != u8 ||
		uint16(-16&0xffff) != u16 ||
		uint32(-32&0xffffffff) != u32 ||
		uint64(-64&0xffffffffffffffff) != u64 ||
		uintptr(42) != uptr {
		t.Error()
	}

	var (
		s        bool
		longbool bool
		long     string
		Xbool    bool
		X        uint
		O        uint
		Ibool    bool
		I        uint
		S        string
		V        string
	)
	outargs, err = OptParse(args, "-s --long= --long --X=%x --X=%x --O=%o --I=%v --I --S=%s --V",
		&s, &longbool, &long, &Xbool, &X, &O, &Ibool, &I, &S, &V)
	if nil != err {
		t.Error()
	}

	if !s || !longbool || "LONG" != long ||
		!Xbool || 0xabc != X || 0777 != O || !Ibool || 0xabc != I ||
		"string" != S || "value" != V {
		t.Error()
	}
}
