ARG BASE_IMAGE=rclone/rclone:latest
FROM ${BASE_IMAGE} as binaries

# build plugin image
FROM alpine:latest

# put rclone in /usr/bin, reserve /usr/local/bin for plugin wrappers
COPY --from=binaries /usr/local/bin/rclone /usr/bin/rclone

RUN mkdir -p /data/config /data/cache /mnt \
 && apk --no-cache add ca-certificates fuse3 tzdata \
 && echo "user_allow_other" >> /etc/fuse.conf \
 && rclone version

ENV RCLONE_CONFIG=/data/config/rclone.conf
ENV RCLONE_CACHE_DIR=/data/cache
ENV RCLONE_BASE_DIR=/mnt
ENV RCLONE_VERBOSE=0

ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV NO_PROXY=

WORKDIR /data
ENTRYPOINT ["rclone", "serve", "docker"]
