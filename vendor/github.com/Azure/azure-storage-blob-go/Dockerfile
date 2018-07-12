FROM golang:1.10

ENV GOPATH /go
ENV PATH ${GOPATH}/bin:$PATH
ENV ACCOUNT_NAME ${ACCOUNT_NAME}
ENV ACCOUNT_KEY ${ACCOUNT_KEY}
RUN go get -u github.com/golang/dep/cmd/dep
RUN go get -u github.com/golang/lint/golint
RUN go get -u github.com/mitchellh/gox



