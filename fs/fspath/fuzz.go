//+build gofuzz

/*
   Fuzz test the Parse function

   Generate corpus

   go test -v -make-corpus

   Install go fuzz

   go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build

   Compile and fuzz

   go-fuzz-build
   go-fuzz

   Tidy up

   rm -rf corpus/ crashers/ suppressions/
   git co ../../go.mod ../../go.sum
*/

package fspath

func Fuzz(data []byte) int {
	path := string(data)
	parsed, err := Parse(path)
	if err != nil {
		return 0
	}
	if parsed.Name == "" {
		if parsed.ConfigString != "" {
			panic("bad ConfigString")
		}
		if parsed.Path != path {
			panic("local path not preserved")
		}
	} else {
		if parsed.ConfigString+":"+parsed.Path != path {
			panic("didn't split properly")
		}
	}
	return 0
}
