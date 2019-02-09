package sftp

import (
	"encoding"
	"sort"
	"sync"
)

// The goal of the packetManager is to keep the outgoing packets in the same
// order as the incoming as is requires by section 7 of the RFC.

type packetManager struct {
	requests    chan orderedPacket
	responses   chan orderedPacket
	fini        chan struct{}
	incoming    orderedPackets
	outgoing    orderedPackets
	sender      packetSender // connection object
	working     *sync.WaitGroup
	packetCount uint32
}

type packetSender interface {
	sendPacket(encoding.BinaryMarshaler) error
}

func newPktMgr(sender packetSender) *packetManager {
	s := &packetManager{
		requests:  make(chan orderedPacket, SftpServerWorkerCount),
		responses: make(chan orderedPacket, SftpServerWorkerCount),
		fini:      make(chan struct{}),
		incoming:  make([]orderedPacket, 0, SftpServerWorkerCount),
		outgoing:  make([]orderedPacket, 0, SftpServerWorkerCount),
		sender:    sender,
		working:   &sync.WaitGroup{},
	}
	go s.controller()
	return s
}

//// packet ordering
func (s *packetManager) newOrderId() uint32 {
	s.packetCount++
	return s.packetCount
}

type orderedRequest struct {
	requestPacket
	orderid uint32
}

func (s *packetManager) newOrderedRequest(p requestPacket) orderedRequest {
	return orderedRequest{requestPacket: p, orderid: s.newOrderId()}
}
func (p orderedRequest) orderId() uint32       { return p.orderid }
func (p orderedRequest) setOrderId(oid uint32) { p.orderid = oid }

type orderedResponse struct {
	responsePacket
	orderid uint32
}

func (s *packetManager) newOrderedResponse(p responsePacket, id uint32,
) orderedResponse {
	return orderedResponse{responsePacket: p, orderid: id}
}
func (p orderedResponse) orderId() uint32       { return p.orderid }
func (p orderedResponse) setOrderId(oid uint32) { p.orderid = oid }

type orderedPacket interface {
	id() uint32
	orderId() uint32
}
type orderedPackets []orderedPacket

func (o orderedPackets) Sort() {
	sort.Slice(o, func(i, j int) bool {
		return o[i].orderId() < o[j].orderId()
	})
}

//// packet registry
// register incoming packets to be handled
func (s *packetManager) incomingPacket(pkt orderedRequest) {
	s.working.Add(1)
	s.requests <- pkt
}

// register outgoing packets as being ready
func (s *packetManager) readyPacket(pkt orderedResponse) {
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
// Keep process packet responses in the order they are received while
// maximizing throughput of file transfers.
func (s *packetManager) workerChan(runWorker func(chan orderedRequest),
) chan orderedRequest {

	// multiple workers for faster read/writes
	rwChan := make(chan orderedRequest, SftpServerWorkerCount)
	for i := 0; i < SftpServerWorkerCount; i++ {
		runWorker(rwChan)
	}

	// single worker to enforce sequential processing of everything else
	cmdChan := make(chan orderedRequest)
	runWorker(cmdChan)

	pktChan := make(chan orderedRequest, SftpServerWorkerCount)
	go func() {
		for pkt := range pktChan {
			switch pkt.requestPacket.(type) {
			case *sshFxpReadPacket, *sshFxpWritePacket:
				s.incomingPacket(pkt)
				rwChan <- pkt
				continue
			case *sshFxpClosePacket:
				// wait for reads/writes to finish when file is closed
				// incomingPacket() call must occur after this
				s.working.Wait()
			}
			s.incomingPacket(pkt)
			// all non-RW use sequential cmdChan
			cmdChan <- pkt
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
			debug("incoming id (oid): %v (%v)", pkt.id(), pkt.orderId())
			s.incoming = append(s.incoming, pkt)
			s.incoming.Sort()
		case pkt := <-s.responses:
			debug("outgoing id (oid): %v (%v)", pkt.id(), pkt.orderId())
			s.outgoing = append(s.outgoing, pkt)
			s.outgoing.Sort()
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
		// debug("incoming: %v", ids(s.incoming))
		// debug("outgoing: %v", ids(s.outgoing))
		if in.orderId() == out.orderId() {
			debug("Sending packet: %v", out.id())
			s.sender.sendPacket(out.(encoding.BinaryMarshaler))
			// pop off heads
			copy(s.incoming, s.incoming[1:])            // shift left
			s.incoming[len(s.incoming)-1] = nil         // clear last
			s.incoming = s.incoming[:len(s.incoming)-1] // remove last
			copy(s.outgoing, s.outgoing[1:])            // shift left
			s.outgoing[len(s.outgoing)-1] = nil         // clear last
			s.outgoing = s.outgoing[:len(s.outgoing)-1] // remove last
		} else {
			break
		}
	}
}

// func oids(o []orderedPacket) []uint32 {
// 	res := make([]uint32, 0, len(o))
// 	for _, v := range o {
// 		res = append(res, v.orderId())
// 	}
// 	return res
// }
// func ids(o []orderedPacket) []uint32 {
// 	res := make([]uint32, 0, len(o))
// 	for _, v := range o {
// 		res = append(res, v.id())
// 	}
// 	return res
// }
