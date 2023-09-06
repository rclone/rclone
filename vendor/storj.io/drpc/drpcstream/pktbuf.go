// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcstream

import (
	"sync"
)

type packetBuffer struct {
	mu   sync.Mutex
	cond sync.Cond
	err  error
	data []byte
	set  bool
	held bool
}

func (pb *packetBuffer) init() {
	pb.cond.L = &pb.mu
}

func (pb *packetBuffer) Close(err error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for pb.held {
		pb.cond.Wait()
	}

	if pb.err == nil {
		pb.data = nil
		pb.set = false
		pb.err = err
		pb.cond.Broadcast()
	}
}

func (pb *packetBuffer) Put(data []byte) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for pb.set && pb.err == nil {
		pb.cond.Wait()
	}
	if pb.err != nil {
		return
	}

	pb.data = data
	pb.set = true
	pb.held = false
	pb.cond.Broadcast()

	for pb.set || pb.held {
		pb.cond.Wait()
	}
}

func (pb *packetBuffer) Get() ([]byte, error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for !pb.set && pb.err == nil {
		pb.cond.Wait()
	}
	if pb.err != nil {
		return nil, pb.err
	}

	pb.held = true
	pb.cond.Broadcast()

	return pb.data, nil
}

func (pb *packetBuffer) Done() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.data = nil
	pb.set = false
	pb.held = false
	pb.cond.Broadcast()
}
