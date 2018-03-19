// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pubsub

import (
	"fmt"
	"math"
	"strings"

	pb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxPayload is the maximum number of bytes to devote to actual ids in
// acknowledgement or modifyAckDeadline requests. A serialized
// AcknowledgeRequest proto has a small constant overhead, plus the size of the
// subscription name, plus 3 bytes per ID (a tag byte and two size bytes). A
// ModifyAckDeadlineRequest has an additional few bytes for the deadline. We
// don't know the subscription name here, so we just assume the size exclusive
// of ids is 100 bytes.
//
// With gRPC there is no way for the client to know the server's max message size (it is
// configurable on the server). We know from experience that it
// it 512K.
const (
	maxPayload       = 512 * 1024
	reqFixedOverhead = 100
	overheadPerID    = 3
	maxSendRecvBytes = 20 * 1024 * 1024 // 20M
)

func convertMessages(rms []*pb.ReceivedMessage) ([]*Message, error) {
	msgs := make([]*Message, 0, len(rms))
	for i, m := range rms {
		msg, err := toMessage(m)
		if err != nil {
			return nil, fmt.Errorf("pubsub: cannot decode the retrieved message at index: %d, message: %+v", i, m)
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func trunc32(i int64) int32 {
	if i > math.MaxInt32 {
		i = math.MaxInt32
	}
	return int32(i)
}

// Logic from https://github.com/GoogleCloudPlatform/google-cloud-java/blob/master/google-cloud-pubsub/src/main/java/com/google/cloud/pubsub/v1/StatusUtil.java.
func isRetryable(err error) bool {
	s, ok := status.FromError(err)
	if !ok { // includes io.EOF, normal stream close, which causes us to reopen
		return true
	}
	switch s.Code() {
	case codes.DeadlineExceeded, codes.Internal, codes.Canceled, codes.ResourceExhausted:
		return true
	case codes.Unavailable:
		return !strings.Contains(s.Message(), "Server shutdownNow invoked")
	default:
		return false
	}
}

// Split req into a prefix that is smaller than maxSize, and a remainder.
func splitRequest(req *pb.StreamingPullRequest, maxSize int) (prefix, remainder *pb.StreamingPullRequest) {
	const int32Bytes = 4

	// Copy all fields before splitting the variable-sized ones.
	remainder = &pb.StreamingPullRequest{}
	*remainder = *req
	// Split message so it isn't too big.
	size := reqFixedOverhead
	i := 0
	for size < maxSize && (i < len(req.AckIds) || i < len(req.ModifyDeadlineAckIds)) {
		if i < len(req.AckIds) {
			size += overheadPerID + len(req.AckIds[i])
		}
		if i < len(req.ModifyDeadlineAckIds) {
			size += overheadPerID + len(req.ModifyDeadlineAckIds[i]) + int32Bytes
		}
		i++
	}

	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	j := i
	if size > maxSize {
		j--
	}
	k := min(j, len(req.AckIds))
	remainder.AckIds = req.AckIds[k:]
	req.AckIds = req.AckIds[:k]
	k = min(j, len(req.ModifyDeadlineAckIds))
	remainder.ModifyDeadlineAckIds = req.ModifyDeadlineAckIds[k:]
	remainder.ModifyDeadlineSeconds = req.ModifyDeadlineSeconds[k:]
	req.ModifyDeadlineAckIds = req.ModifyDeadlineAckIds[:k]
	req.ModifyDeadlineSeconds = req.ModifyDeadlineSeconds[:k]
	return req, remainder
}
