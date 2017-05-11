// +build !go1.8

package sftp

import "sort"

// for sorting/ordering outgoing
type responsePackets []responsePacket

func (r responsePackets) Len() int           { return len(r) }
func (r responsePackets) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r responsePackets) Less(i, j int) bool { return r[i].id() < r[j].id() }
func (r responsePackets) Sort()              { sort.Sort(r) }

// for sorting/ordering incoming
type requestPacketIDs []uint32

func (r requestPacketIDs) Len() int           { return len(r) }
func (r requestPacketIDs) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r requestPacketIDs) Less(i, j int) bool { return r[i] < r[j] }
func (r requestPacketIDs) Sort()              { sort.Sort(r) }
