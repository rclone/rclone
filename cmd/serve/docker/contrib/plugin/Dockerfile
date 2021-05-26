ARG BASE_IMAGE=rclone/rclone:latest
ARG BUILD_PLATFORM=linux/amd64
ARG TARGET_PLATFORM=linux/amd64

# temporary build image
FROM --platform=${BUILD_PLATFORM} golang:alpine AS BUILD_ENV

COPY . /src
WORKDIR /src

RUN apk add --no-cache make git bash && \
    CGO_ENABLED=0 \
    GOARCH=$(echo ${TARGET_PLATFORM} | cut -d '/' -f2) \
    make rclone

# plugin image
FROM ${BASE_IMAGE}

COPY --from=BUILD_ENV /src/rclone /usr/local/bin/rclone

RUN mkdir -p /data/config /data/cache /mnt \
 && /usr/local/bin/rclone version

ENV RCLONE_CONFIG=/data/config/rclone.conf
ENV RCLONE_CACHE_DIR=/data/cache
ENV RCLONE_BASE_DIR=/mnt
ENV RCLONE_VERBOSE=0

WORKDIR /data
ENTRYPOINT ["/usr/local/bin/rclone"]
CMD ["serve", "docker"]
