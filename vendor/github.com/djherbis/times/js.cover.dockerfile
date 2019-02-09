FROM golang:1.11

RUN curl -sL https://deb.nodesource.com/setup_8.x | bash
RUN apt-get install --yes nodejs

WORKDIR /go/src/github.com/djherbis/times
COPY . .

RUN GOOS=js GOARCH=wasm go test -covermode=count -coverprofile=profile.cov -exec="$(go env GOROOT)/misc/wasm/go_js_wasm_exec"