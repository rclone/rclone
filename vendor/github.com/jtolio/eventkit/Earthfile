VERSION 0.6
FROM golang:1.18
WORKDIR /go/eventkit

lint:
    RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        go install honnef.co/go/tools/cmd/staticcheck@2022.1.3
    RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.2
    COPY . .
    RUN golangci-lint run
    RUN staticcheck ./...

test:
   COPY . .
       RUN --mount=type=cache,target=/root/.cache/go-build \
           --mount=type=cache,target=/go/pkg/mod \
           go test ./...

check-format:
   COPY . .
   RUN bash -c 'mkdir build || true'
   RUN bash -c '[[ $(git status --short) == "" ]] || (echo "Before formatting, please commit all your work!!! (Formatter will format only last commit)" && exit -1)'
   RUN git show --name-only --pretty=format: | grep ".go" | xargs -n1 gofmt -s -w
   RUN git diff > build/format.patch
   SAVE ARTIFACT build/format.patch

format:
   LOCALLY
   COPY +check-format/format.patch build/format.patch
   RUN git apply --allow-empty build/format.patch
   RUN git status
