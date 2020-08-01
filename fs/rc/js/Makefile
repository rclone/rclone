build:
	GOARCH=wasm GOOS=js go build -o rclone.wasm

serve:	build
	go run serve.go
