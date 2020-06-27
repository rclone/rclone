//+build !noasm,!appengine,gc

// Copyright (c) 2020 MinIO Inc. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package md5simd

import (
	"encoding/binary"
	"fmt"
	"runtime"

	"github.com/klauspost/cpuid"
)

// MD5 initialization constants
const (
	// Lanes is the number of concurrently calculated hashes.
	Lanes = 16

	init0 = 0x67452301
	init1 = 0xefcdab89
	init2 = 0x98badcfe
	init3 = 0x10325476
)

// md5ServerUID - Does not start at 0 but next multiple of 16 so as to be able to
// differentiate with default initialisation value of 0
const md5ServerUID = Lanes

const buffersPerLane = 3

// Message to send across input channel
type blockInput struct {
	uid   uint64
	msg   []byte
	sumCh chan sumResult
	reset bool
}

type sumResult struct {
	digest [Size]byte
}

type lanesInfo [Lanes]blockInput

// md5Server - Type to implement parallel handling of MD5 invocations
type md5Server struct {
	uidCounter   uint64
	cycle        chan uint64           // client with uid has update.
	newInput     chan newClient        // Add new client.
	digests      map[uint64][Size]byte // Map of uids to (interim) digest results
	maskRounds16 [16]maskRounds        // Pre-allocated static array for max 16 rounds
	maskRounds8a [8]maskRounds         // Pre-allocated static array for max 8 rounds (1st AVX2 core)
	maskRounds8b [8]maskRounds         // Pre-allocated static array for max 8 rounds (2nd AVX2 core)
	allBufs      []byte                // Preallocated buffer.
	buffers      chan []byte           // Preallocated buffers, sliced from allBufs.
}

// NewServer - Create new object for parallel processing handling
func NewServer() Server {
	if !cpuid.CPU.AVX2() {
		return &fallbackServer{}
	}
	md5srv := &md5Server{}
	md5srv.digests = make(map[uint64][Size]byte)
	md5srv.newInput = make(chan newClient, Lanes)
	md5srv.cycle = make(chan uint64, Lanes*10)
	md5srv.uidCounter = md5ServerUID - 1
	md5srv.allBufs = make([]byte, 32+buffersPerLane*Lanes*internalBlockSize)
	md5srv.buffers = make(chan []byte, buffersPerLane*Lanes)
	// Fill buffers.
	for i := 0; i < buffersPerLane*Lanes; i++ {
		s := 32 + i*internalBlockSize
		md5srv.buffers <- md5srv.allBufs[s : s+internalBlockSize : s+internalBlockSize]
	}

	// Start a single thread for reading from the input channel
	go md5srv.process(md5srv.newInput)
	return md5srv
}

type newClient struct {
	uid   uint64
	input chan blockInput
}

// process - Sole handler for reading from the input channel.
func (s *md5Server) process(newClients chan newClient) {
	// To fill up as many lanes as possible:
	//
	// 1. Wait for a cycle id.
	// 2. If not already in a lane, add, otherwise leave on channel
	// 3. Start timer
	// 4. Check if lanes is full, if so, goto 10 (process).
	// 5. If timeout, goto 10.
	// 6. Wait for new id (goto 2)  or timeout (goto 10).
	// 10. Process.
	// 11. Check all input if there is already input, if so add to lanes.
	// 12. Goto 1

	// lanes contains the lanes.
	var lanes lanesInfo
	// lanesFilled contains the number of filled lanes for current cycle.
	var lanesFilled int
	// clients contains active clients
	var clients = make(map[uint64]chan blockInput, Lanes)

	addToLane := func(uid uint64) {
		cl, ok := clients[uid]
		if !ok {
			// Unknown client. Maybe it was already removed.
			return
		}
		// Check if we already have it.
		for _, lane := range lanes[:lanesFilled] {
			if lane.uid == uid {
				return
			}
		}
		// Continue until we get a block or there is nothing on channel
		for {
			select {
			case block, ok := <-cl:
				if !ok {
					// Client disconnected
					delete(clients, block.uid)
					return
				}
				if block.uid != uid {
					panic(fmt.Errorf("uid mismatch, %d (block) != %d (client)", block.uid, uid))
				}
				// If reset message, reset and we're done
				if block.reset {
					delete(s.digests, uid)
					continue
				}

				// If requesting sum, we will need to maintain state.
				if block.sumCh != nil {
					var dig digest
					d, ok := s.digests[uid]
					if ok {
						dig.s[0] = binary.LittleEndian.Uint32(d[0:4])
						dig.s[1] = binary.LittleEndian.Uint32(d[4:8])
						dig.s[2] = binary.LittleEndian.Uint32(d[8:12])
						dig.s[3] = binary.LittleEndian.Uint32(d[12:16])
					} else {
						dig.s[0], dig.s[1], dig.s[2], dig.s[3] = init0, init1, init2, init3
					}

					sum := sumResult{}
					// Add end block to current digest.
					blockGeneric(&dig, block.msg)

					binary.LittleEndian.PutUint32(sum.digest[0:], dig.s[0])
					binary.LittleEndian.PutUint32(sum.digest[4:], dig.s[1])
					binary.LittleEndian.PutUint32(sum.digest[8:], dig.s[2])
					binary.LittleEndian.PutUint32(sum.digest[12:], dig.s[3])
					block.sumCh <- sum
					if block.msg != nil {
						s.buffers <- block.msg
					}
					continue
				}
				if len(block.msg) == 0 {
					continue
				}
				lanes[lanesFilled] = block
				lanesFilled++
				return
			default:
				return
			}
		}
	}
	addNewClient := func(cl newClient) {
		if _, ok := clients[cl.uid]; ok {
			panic("internal error: duplicate client registration")
		}
		clients[cl.uid] = cl.input
	}

	allLanesFilled := func() bool {
		return lanesFilled == Lanes || lanesFilled >= len(clients)
	}

	for {
		// Step 1.
		for lanesFilled == 0 {
			select {
			case cl, ok := <-newClients:
				if !ok {
					return
				}
				addNewClient(cl)
				// Check if it already sent a payload.
				addToLane(cl.uid)
				continue
			case uid := <-s.cycle:
				addToLane(uid)
			}
		}

	fillLanes:
		for !allLanesFilled() {
			select {
			case cl, ok := <-newClients:
				if !ok {
					return
				}
				addNewClient(cl)

			case uid := <-s.cycle:
				addToLane(uid)
			default:
				// Nothing more queued...
				break fillLanes
			}
		}

		// If we did not fill all lanes, check if there is more waiting
		if !allLanesFilled() {
			runtime.Gosched()
			for uid := range clients {
				addToLane(uid)
				if allLanesFilled() {
					break
				}
			}
		}
		if false {
			if !allLanesFilled() {
				fmt.Println("Not all lanes filled", lanesFilled, "of", len(clients))
				//pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			} else if true {
				fmt.Println("all lanes filled")
			}
		}
		// Process the lanes we could collect
		s.blocks(lanes[:lanesFilled])

		// Clear lanes...
		lanesFilled = 0
		// Add all current queued
		for uid := range clients {
			addToLane(uid)
			if allLanesFilled() {
				break
			}
		}
	}
}

func (s *md5Server) Close() {
	if s.newInput != nil {
		close(s.newInput)
		s.newInput = nil
	}
}

// Invoke assembly and send results back
func (s *md5Server) blocks(lanes []blockInput) {
	inputs := [16][]byte{}
	for i := range lanes {
		inputs[i] = lanes[i].msg
	}

	// Collect active digests...
	state := s.getDigests(lanes)
	// Process all lanes...
	s.blockMd5_x16(&state, inputs, len(lanes) <= 8)

	for i, lane := range lanes {
		uid := lane.uid
		dig := [Size]byte{}
		binary.LittleEndian.PutUint32(dig[0:], state.v0[i])
		binary.LittleEndian.PutUint32(dig[4:], state.v1[i])
		binary.LittleEndian.PutUint32(dig[8:], state.v2[i])
		binary.LittleEndian.PutUint32(dig[12:], state.v3[i])

		s.digests[uid] = dig
		if lane.msg != nil {
			s.buffers <- lane.msg
		}
		lanes[i] = blockInput{}
	}
}

func (s *md5Server) getDigests(lanes []blockInput) (d digest16) {
	for i, lane := range lanes {
		a, ok := s.digests[lane.uid]
		if ok {
			d.v0[i] = binary.LittleEndian.Uint32(a[0:4])
			d.v1[i] = binary.LittleEndian.Uint32(a[4:8])
			d.v2[i] = binary.LittleEndian.Uint32(a[8:12])
			d.v3[i] = binary.LittleEndian.Uint32(a[12:16])
		} else {
			d.v0[i] = init0
			d.v1[i] = init1
			d.v2[i] = init2
			d.v3[i] = init3
		}
	}
	return
}
