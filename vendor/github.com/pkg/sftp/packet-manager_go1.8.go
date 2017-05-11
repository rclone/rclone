// +build go1.8

package sftp

import "sort"

type responsePackets []responsePacket

func (r responsePackets) Sort() {
	sort.Slice(r, func(i, j int) bool {
		return r[i].id() < r[j].id()
	})
}

type requestPacketIDs []uint32

func (r requestPacketIDs) Sort() {
	sort.Slice(r, func(i, j int) bool {
		return r[i] < r[j]
	})
}
