ARG ALPINE_VERSION=3.18
ARG GO_VERSION=1.21
ARG XCPUTRANSLATE_VERSION=v0.6.0
ARG BUILDPLATFORM=linux/amd64

FROM --platform=${BUILDPLATFORM} qmcgaw/xcputranslate:${XCPUTRANSLATE_VERSION} AS xcputranslate

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder
COPY --from=xcputranslate /xcputranslate /usr/local/bin/xcputranslate
RUN apk --update add git make bash
ENV CGO_ENABLED=0
WORKDIR /tmp/gobuild
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETPLATFORM
RUN GOARCH="$(xcputranslate translate -field arch -targetplatform ${TARGETPLATFORM})" \
  GOARM="$(xcputranslate translate -field arm -targetplatform ${TARGETPLATFORM})" \
  make rclone

FROM alpine:${ALPINE_VERSION}
RUN apk --no-cache add ca-certificates fuse3 tzdata && \
  echo "user_allow_other" >> /etc/fuse.conf
RUN addgroup -g 1009 rclone && adduser -u 1009 -Ds /bin/sh -G rclone rclone
ENV XDG_CONFIG_HOME=/config
WORKDIR /data
ENTRYPOINT /usr/local/bin/rclone
COPY --from=builder /tmp/gobuild/rclone /usr/local/bin/rclone
