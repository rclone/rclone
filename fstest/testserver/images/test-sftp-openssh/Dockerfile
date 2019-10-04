# A very minimal sftp server for integration testing rclone
FROM alpine:latest

# User rclone, password password
RUN \
    apk add openssh && \
    ssh-keygen -A && \
    adduser -D rclone && \
    echo "rclone:password" | chpasswd 

ENTRYPOINT [ "/usr/sbin/sshd", "-D" ]
