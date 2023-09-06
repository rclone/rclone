//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package blockblob

import (
	"fmt"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

// mmb is a memory mapped buffer
type mmb []byte

// newMMB creates a new memory mapped buffer with the specified size
func newMMB(size int64) (mmb, error) {
	const InvalidHandleValue = ^uintptr(0) // -1

	prot, access := uint32(syscall.PAGE_READWRITE), uint32(syscall.FILE_MAP_WRITE)
	hMMF, err := syscall.CreateFileMapping(syscall.Handle(InvalidHandleValue), nil, prot, uint32(size>>32), uint32(size&0xffffffff), nil)
	if err != nil {
		return nil, os.NewSyscallError("CreateFileMapping", err)
	}
	defer func() {
		_ = syscall.CloseHandle(hMMF)
	}()

	addr, err := syscall.MapViewOfFile(hMMF, access, 0, 0, uintptr(size))
	if err != nil {
		return nil, os.NewSyscallError("MapViewOfFile", err)
	}

	m := mmb{}
	h := (*reflect.SliceHeader)(unsafe.Pointer(&m))
	h.Data = addr
	h.Len = int(size)
	h.Cap = h.Len
	return m, nil
}

// delete cleans up the memory mapped buffer
func (m *mmb) delete() {
	addr := uintptr(unsafe.Pointer(&(([]byte)(*m)[0])))
	*m = mmb{}
	err := syscall.UnmapViewOfFile(addr)
	if err != nil {
		// if we get here, there is likely memory corruption.
		// please open an issue https://github.com/Azure/azure-sdk-for-go/issues
		panic(fmt.Sprintf("UnmapViewOfFile error: %v", err))
	}
}
