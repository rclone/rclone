# Rclone as WASM

This directory contains files to use the rclone rc as a library in the browser.

This works by compiling rclone to WASM and loading that in via javascript.

This contains the following files

- `index.html` - test web page to load the module
- `loader.js` - java script to load the module - see here for usage
- `main.go` - main go code exporting the rclone rc
- `Makefile` - test makefile
- `README.md` - this readme
- `serve.go` - test program to serve the web page
- `wasm_exec.js` - interface code from the go source - don't edit

## Compiling

This can be compiled by using `make` or alternatively `GOARCH=wasm GOOS=js go build -o rclone.wasm`

## Running

Run the test server with `make serve` and examine the page at
http://localhost:3000/ - look at the javascript console and look at
the end of `loader.js` for how that works.
