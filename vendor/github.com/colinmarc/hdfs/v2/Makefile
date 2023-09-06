HADOOP_COMMON_PROTOS = $(shell find internal/protocol/hadoop_common -name '*.proto')
HADOOP_HDFS_PROTOS = $(shell find internal/protocol/hadoop_hdfs -name '*.proto')
GENERATED_PROTOS = $(shell echo "$(HADOOP_HDFS_PROTOS) $(HADOOP_COMMON_PROTOS)" | sed 's/\.proto/\.pb\.go/g')
SOURCES = $(shell find . -name '*.go') $(GENERATED_PROTOS)

# Protobuf needs one of these for every 'import "foo.proto"' in .protoc files.
PROTO_MAPPING = MSecurity.proto=github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_common

TAG ?= $(shell git describe --tag)
ARCH = $(shell go env GOOS)-$(shell go env GOARCH)
RELEASE_NAME = gohdfs-$(TAG)-$(ARCH)

all: hdfs

%.pb.go: $(HADOOP_HDFS_PROTOS) $(HADOOP_COMMON_PROTOS)
	echo $(HADOOP_COMMON_PROTOS)
	protoc --go_out=internal/protocol/hadoop_common --go_opt=paths=source_relative -Iinternal/protocol/hadoop_common -Iinternal/protocol/hadoop_hdfs $(HADOOP_COMMON_PROTOS)
	protoc --go_out=internal/protocol/hadoop_hdfs --go_opt=paths=source_relative -Iinternal/protocol/hadoop_common -Iinternal/protocol/hadoop_hdfs $(HADOOP_HDFS_PROTOS)

clean-protos:
	find . -name *.pb.go | xargs rm

hdfs: clean $(SOURCES)
	go build -ldflags "-X main.version=$(TAG)" ./cmd/hdfs

test: hdfs
	go test -v -race -timeout 30s ./...
	bats ./cmd/hdfs/test/*.bats

clean:
	rm -f ./hdfs
	rm -rf gohdfs-*

release: hdfs
	mkdir -p $(RELEASE_NAME)
	cp hdfs README.md LICENSE.txt cmd/hdfs/bash_completion $(RELEASE_NAME)/
	tar -cvzf $(RELEASE_NAME).tar.gz $(RELEASE_NAME)

.PHONY: clean clean-protos install test release
