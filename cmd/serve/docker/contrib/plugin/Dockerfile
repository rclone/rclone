ARG BASE_IMAGE=rclone/rclone:latest
ARG BUILD_PLATFORM=linux/amd64
ARG TARGET_PLATFORM=linux/amd64

# plugin image
FROM ${BASE_IMAGE} as binaries
FROM alpine:latest

COPY --from=binaries /usr/local/bin/rclone /usr/local/bin/rclone
RUN mkdir -p /data/config /data/cache /mnt \
 && /usr/local/bin/rclone version

ENV RCLONE_CONFIG=/data/config/rclone.conf
ENV RCLONE_CACHE_DIR=/data/cache
ENV RCLONE_BASE_DIR=/mnt
ENV RCLONE_VERBOSE=0

WORKDIR /data
ENTRYPOINT ["/usr/local/bin/rclone"]
CMD ["serve", "docker"]
