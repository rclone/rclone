package sftp

import (
	"encoding"
	"sync"
)

// The goal of the packetManager is to keep the outgoing packets in the same
// order as the incoming. This is due to some sftp clients requiring this
// behavior (eg. winscp).

type packetSender interface {
	sendPacket(encoding.BinaryMarshaler) error
}

type packetManager struct {
	requests  chan requestPacket
	responses chan responsePacket
	fini      chan struct{}
	incoming  requestPacketIDs
	outgoing  responsePackets
	sender    packetSender // connection object
	working   *sync.WaitGroup
}

func newPktMgr(sender packetSender) *packetManager {
	s := &packetManager{
		requests:  make(chan requestPacket, SftpServerWorkerCount),
		responses: make(chan responsePacket, SftpServerWorkerCount),
		fini:      make(chan struct{}),
		incoming:  make([]uint32, 0, SftpServerWorkerCount),
		outgoing:  make([]responsePacket, 0, SftpServerWorkerCount),
		sender:    sender,
		working:   &sync.WaitGroup{},
	}
	go s.controller()
	return s
}

// register incoming packets to be handled
// send id of 0 for packets without id
func (s *packetManager) incomingPacket(pkt requestPacket) {
	s.working.Add(1)
	s.requests <- pkt // buffer == SftpServerWorkerCount
}

// register outgoing packets as being ready
func (s *packetManager) readyPacket(pkt responsePacket) {
	s.responses <- pkt
	s.working.Done()
}

// shut down packetManager controller
func (s *packetManager) close() {
	// pause until current packets are processed
	s.working.Wait()
	close(s.fini)
}

// Passed a worker function, returns a channel for incoming packets.
// The goal is to process packets in the order they are received as is
// requires by section 7 of the RFC, while maximizing throughput of file
// transfers.
func (s *packetManager) workerChan(runWorker func(requestChan)) requestChan {

	rwChan := make(chan requestPacket, SftpServerWorkerCount)
	for i := 0; i < SftpServerWorkerCount; i++ {
		runWorker(rwChan)
	}

	cmdChan := make(chan requestPacket)
	runWorker(cmdChan)

	pktChan := make(chan requestPacket, SftpServerWorkerCount)
	go func() {
		// start with cmdChan
		curChan := cmdChan
		for pkt := range pktChan {
			// on file open packet, switch to rwChan
			switch pkt.(type) {
			case *sshFxpOpenPacket:
				curChan = rwChan
			// on file close packet, switch back to cmdChan
			// after waiting for any reads/writes to finish
			case *sshFxpClosePacket:
				// wait for rwChan to finish
				s.working.Wait()
				// stop using rwChan
				curChan = cmdChan
			}
			s.incomingPacket(pkt)
			curChan <- pkt
		}
		close(rwChan)
		close(cmdChan)
		s.close()
	}()

	return pktChan
}

// process packets
func (s *packetManager) controller() {
	for {
		select {
		case pkt := <-s.requests:
			debug("incoming id: %v", pkt.id())
			s.incoming = append(s.incoming, pkt.id())
			if len(s.incoming) > 1 {
				s.incoming.Sort()
			}
		case pkt := <-s.responses:
			debug("outgoing pkt: %v", pkt.id())
			s.outgoing = append(s.outgoing, pkt)
			if len(s.outgoing) > 1 {
				s.outgoing.Sort()
			}
		case <-s.fini:
			return
		}
		s.maybeSendPackets()
	}
}

// send as many packets as are ready
func (s *packetManager) maybeSendPackets() {
	for {
		if len(s.outgoing) == 0 || len(s.incoming) == 0 {
			debug("break! -- outgoing: %v; incoming: %v",
				len(s.outgoing), len(s.incoming))
			break
		}
		out := s.outgoing[0]
		in := s.incoming[0]
		// 		debug("incoming: %v", s.incoming)
		// 		debug("outgoing: %v", outfilter(s.outgoing))
		if in == out.id() {
			s.sender.sendPacket(out)
			// pop off heads
			copy(s.incoming, s.incoming[1:])            // shift left
			s.incoming = s.incoming[:len(s.incoming)-1] // remove last
			copy(s.outgoing, s.outgoing[1:])            // shift left
			s.outgoing = s.outgoing[:len(s.outgoing)-1] // remove last
		} else {
			break
		}
	}
}

//func outfilter(o []responsePacket) []uint32 {
//	res := make([]uint32, 0, len(o))
//	for _, v := range o {
//		res = append(res, v.id())
//	}
//	return res
//}
