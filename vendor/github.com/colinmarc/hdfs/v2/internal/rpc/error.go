package rpc

import (
	"fmt"

	hadoop "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_common"
)

// NamenodeError represents an interepreted error from the Namenode, including
// the error code and the java backtrace. It implements hdfs.Error.
type NamenodeError struct {
	method    string
	code      int
	exception string
	message   string
}

func (err *NamenodeError) Method() string {
	return err.method
}

func (err *NamenodeError) Desc() string {
	return hadoop.RpcResponseHeaderProto_RpcErrorCodeProto_name[int32(err.code)]
}

func (err *NamenodeError) Exception() string {
	return err.exception
}

func (err *NamenodeError) Message() string {
	return err.message
}

func (err *NamenodeError) Error() string {
	s := fmt.Sprintf("%s call failed with %s", err.method, err.Desc())
	if err.exception != "" {
		s += fmt.Sprintf(" (%s)", err.exception)
	}

	return s
}

type DatanodeError struct {
}
