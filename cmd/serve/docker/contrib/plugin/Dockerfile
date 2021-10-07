ARG BASE_IMAGE=rclone/rclone:latest

# plugin image
FROM ${BASE_IMAGE}

RUN mkdir -p /data/config /data/cache /mnt \
 && /usr/local/bin/rclone version

ENV RCLONE_CONFIG=/data/config/rclone.conf
ENV RCLONE_CACHE_DIR=/data/cache
ENV RCLONE_BASE_DIR=/mnt
ENV RCLONE_VERBOSE=0

WORKDIR /data
CMD ["serve", "docker"]