FROM golang

COPY . /go/src/github.com/rclone/rclone/
WORKDIR /go/src/github.com/rclone/rclone/

RUN go build -v
RUN ./rclone version

ENTRYPOINT [ "./rclone" ]
