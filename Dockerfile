FROM alpine:latest

RUN apk --no-cache add ca-certificates fuse3 tzdata unzip coreutils && \
    echo "user_allow_other" >> /etc/fuse.conf

ARG TARGETARCH

ARG TARGETVARIANT

ARG VERSION

COPY build/rclone-${VERSION}-linux-${TARGETARCH}${TARGETVARIANT:+-$TARGETVARIANT}.zip /tmp/rclone.zip

RUN unzip /tmp/rclone.zip -d /tmp && \
    mv /tmp/rclone-*-linux-${TARGETARCH}${TARGETVARIANT:+-$TARGETVARIANT}/rclone /usr/local/bin/rclone && \
    chmod +x /usr/local/bin/rclone && \
    rm -rf /tmp/rclone* && \
    apk del unzip

RUN addgroup -g 1009 rclone && adduser -u 1009 -Ds /bin/sh -G rclone rclone

ENTRYPOINT [ "rclone" ]

WORKDIR /data

ENV XDG_CONFIG_HOME=/config