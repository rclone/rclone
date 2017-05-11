package syscallx

/* This is the source file for syscallx_darwin_*.go, to regenerate run

   ./generate

*/

// cannot use dest []byte here because OS X getxattr really wants a
// NULL to trigger size probing, size==0 is not enough
//
//sys getxattr(path string, attr string, dest *byte, size int, position uint32, options int) (sz int, err error)

func Getxattr(path string, attr string, dest []byte) (sz int, err error) {
	var destp *byte
	if len(dest) > 0 {
		destp = &dest[0]
	}
	return getxattr(path, attr, destp, len(dest), 0, 0)
}

//sys listxattr(path string, dest []byte, options int) (sz int, err error)

func Listxattr(path string, dest []byte) (sz int, err error) {
	return listxattr(path, dest, 0)
}

//sys setxattr(path string, attr string, data []byte, position uint32, flags int) (err error)

func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return setxattr(path, attr, data, 0, flags)
}

//sys removexattr(path string, attr string, options int) (err error)

func Removexattr(path string, attr string) (err error) {
	return removexattr(path, attr, 0)
}
