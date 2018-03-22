package sftp

import (
	"encoding"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type _testSender struct {
	sent chan encoding.BinaryMarshaler
}

func newTestSender() *_testSender {
	return &_testSender{make(chan encoding.BinaryMarshaler)}
}

func (s _testSender) sendPacket(p encoding.BinaryMarshaler) error {
	s.sent <- p
	return nil
}

type fakepacket uint32

func (fakepacket) MarshalBinary() ([]byte, error) {
	return []byte{}, nil
}

func (fakepacket) UnmarshalBinary([]byte) error {
	return nil
}

func (f fakepacket) id() uint32 {
	return uint32(f)
}

type pair struct {
	in  fakepacket
	out fakepacket
}

// basic test
var ttable1 = []pair{
	pair{fakepacket(0), fakepacket(0)},
	pair{fakepacket(1), fakepacket(1)},
	pair{fakepacket(2), fakepacket(2)},
	pair{fakepacket(3), fakepacket(3)},
}

// outgoing packets out of order
var ttable2 = []pair{
	pair{fakepacket(0), fakepacket(0)},
	pair{fakepacket(1), fakepacket(4)},
	pair{fakepacket(2), fakepacket(1)},
	pair{fakepacket(3), fakepacket(3)},
	pair{fakepacket(4), fakepacket(2)},
}

// incoming packets out of order
var ttable3 = []pair{
	pair{fakepacket(2), fakepacket(0)},
	pair{fakepacket(1), fakepacket(1)},
	pair{fakepacket(3), fakepacket(2)},
	pair{fakepacket(0), fakepacket(3)},
}

var tables = [][]pair{ttable1, ttable2, ttable3}

func TestPacketManager(t *testing.T) {
	sender := newTestSender()
	s := newPktMgr(sender)

	for i := range tables {
		table := tables[i]
		for _, p := range table {
			s.incomingPacket(p.in)
		}
		for _, p := range table {
			s.readyPacket(p.out)
		}
		for i := 0; i < len(table); i++ {
			pkt := <-sender.sent
			id := pkt.(fakepacket).id()
			assert.Equal(t, id, uint32(i))
		}
	}
	s.close()
}

func (p sshFxpRemovePacket) String() string {
	return fmt.Sprintf("RmPct:%d", p.ID)
}
func (p sshFxpOpenPacket) String() string {
	return fmt.Sprintf("OpPct:%d", p.ID)
}
func (p sshFxpWritePacket) String() string {
	return fmt.Sprintf("WrPct:%d", p.ID)
}
func (p sshFxpClosePacket) String() string {
	return fmt.Sprintf("ClPct:%d", p.ID)
}
