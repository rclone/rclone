FROM golang:alpine AS builder

ARG CGO_ENABLED=0

WORKDIR /go/src/github.com/rclone/rclone/

RUN echo "**** Set Go Environment Variables ****" && \
    go env -w GOCACHE=/root/.cache/go-build

RUN echo "**** Install Dependencies ****" && \
    apk add --no-cache \
        make \
        bash \
        gawk \
        git

COPY go.mod .
COPY go.sum .

RUN echo "**** Download Go Dependencies ****" && \
    go mod download -x

RUN echo "**** Verify Go Dependencies ****" && \
    go mod verify

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    echo "**** Build Binary ****" && \
    make

RUN echo "**** Print Version Binary ****" && \
    ./rclone version

# Begin final image
FROM alpine:latest

RUN echo "**** Install Dependencies ****" && \
    apk add --no-cache \
        ca-certificates \
        fuse3 \
        tzdata && \
    echo "Enable user_allow_other in fuse" && \
    echo "user_allow_other" >> /etc/fuse.conf

COPY --from=builder /go/src/github.com/rclone/rclone/rclone /usr/local/bin/

RUN addgroup -g 1009 rclone && adduser -u 1009 -Ds /bin/sh -G rclone rclone

ENTRYPOINT [ "rclone" ]

WORKDIR /data
ENV XDG_CONFIG_HOME=/config
