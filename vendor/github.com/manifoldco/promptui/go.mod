module github.com/manifoldco/promptui

require (
	github.com/alecthomas/gometalinter v2.0.11+incompatible
	github.com/chzyer/logex v1.1.10 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/chzyer/test v0.0.0-20180213035817-a1ea475d72b1 // indirect
	github.com/client9/misspell v0.3.4
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/lint v0.0.0-20181026193005-c67002cb31c3
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf // indirect
	github.com/gordonklaus/ineffassign v0.0.0-20180909121442-1003c8bd00dc
	github.com/juju/ansiterm v0.0.0-20180109212912-720a0952cc2a
	github.com/kr/pretty v0.1.0 // indirect
	github.com/lunixbochs/vtclean v0.0.0-20180621232353-2d01aacdc34a // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.2.2 // indirect
	github.com/tsenart/deadcode v0.0.0-20160724212837-210d2dc333e9
	golang.org/x/lint v0.0.0-20181026193005-c67002cb31c3 // indirect
	golang.org/x/sys v0.0.0-20181122145206-62eef0e2fa9b // indirect
	golang.org/x/tools v0.0.0-20181122213734-04b5d21e00f1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

// This version of kingpin is incompatible with the released version of
// gometalinter until the next release of gometalinter, and possibly until it
// has go module support, we'll need this exclude, and perhaps some more.
//
// After that point, we should be able to remove it.
exclude gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20180810215634-df19058c872c
