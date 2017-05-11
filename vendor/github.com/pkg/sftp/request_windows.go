package sftp

import "syscall"

func fakeFileInfoSys() interface{} {
	return syscall.Win32FileAttributeData{}
}

func testOsSys(sys interface{}) error {
	return nil
}
