package net

import (
	"context"
	"encoding/json"
	"net"

	"github.com/shirou/gopsutil/v3/internal/common"
)

var invoke common.Invoker = common.Invoke{}

type IOCountersStat struct {
	Name        string `json:"name"`        // interface name
	BytesSent   uint64 `json:"bytesSent"`   // number of bytes sent
	BytesRecv   uint64 `json:"bytesRecv"`   // number of bytes received
	PacketsSent uint64 `json:"packetsSent"` // number of packets sent
	PacketsRecv uint64 `json:"packetsRecv"` // number of packets received
	Errin       uint64 `json:"errin"`       // total number of errors while receiving
	Errout      uint64 `json:"errout"`      // total number of errors while sending
	Dropin      uint64 `json:"dropin"`      // total number of incoming packets which were dropped
	Dropout     uint64 `json:"dropout"`     // total number of outgoing packets which were dropped (always 0 on OSX and BSD)
	Fifoin      uint64 `json:"fifoin"`      // total number of FIFO buffers errors while receiving
	Fifoout     uint64 `json:"fifoout"`     // total number of FIFO buffers errors while sending
}

// Addr is implemented compatibility to psutil
type Addr struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}

type ConnectionStat struct {
	Fd     uint32  `json:"fd"`
	Family uint32  `json:"family"`
	Type   uint32  `json:"type"`
	Laddr  Addr    `json:"localaddr"`
	Raddr  Addr    `json:"remoteaddr"`
	Status string  `json:"status"`
	Uids   []int32 `json:"uids"`
	Pid    int32   `json:"pid"`
}

// System wide stats about different network protocols
type ProtoCountersStat struct {
	Protocol string           `json:"protocol"`
	Stats    map[string]int64 `json:"stats"`
}

// NetInterfaceAddr is designed for represent interface addresses
type InterfaceAddr struct {
	Addr string `json:"addr"`
}

// InterfaceAddrList is a list of InterfaceAddr
type InterfaceAddrList []InterfaceAddr

type InterfaceStat struct {
	Index        int               `json:"index"`
	MTU          int               `json:"mtu"`          // maximum transmission unit
	Name         string            `json:"name"`         // e.g., "en0", "lo0", "eth0.100"
	HardwareAddr string            `json:"hardwareAddr"` // IEEE MAC-48, EUI-48 and EUI-64 form
	Flags        []string          `json:"flags"`        // e.g., FlagUp, FlagLoopback, FlagMulticast
	Addrs        InterfaceAddrList `json:"addrs"`
}

// InterfaceStatList is a list of InterfaceStat
type InterfaceStatList []InterfaceStat

type FilterStat struct {
	ConnTrackCount int64 `json:"connTrackCount"`
	ConnTrackMax   int64 `json:"connTrackMax"`
}

// ConntrackStat has conntrack summary info
type ConntrackStat struct {
	Entries       uint32 `json:"entries"`       // Number of entries in the conntrack table
	Searched      uint32 `json:"searched"`      // Number of conntrack table lookups performed
	Found         uint32 `json:"found"`         // Number of searched entries which were successful
	New           uint32 `json:"new"`           // Number of entries added which were not expected before
	Invalid       uint32 `json:"invalid"`       // Number of packets seen which can not be tracked
	Ignore        uint32 `json:"ignore"`        // Packets seen which are already connected to an entry
	Delete        uint32 `json:"delete"`        // Number of entries which were removed
	DeleteList    uint32 `json:"deleteList"`    // Number of entries which were put to dying list
	Insert        uint32 `json:"insert"`        // Number of entries inserted into the list
	InsertFailed  uint32 `json:"insertFailed"`  // # insertion attempted but failed (same entry exists)
	Drop          uint32 `json:"drop"`          // Number of packets dropped due to conntrack failure.
	EarlyDrop     uint32 `json:"earlyDrop"`     // Dropped entries to make room for new ones, if maxsize reached
	IcmpError     uint32 `json:"icmpError"`     // Subset of invalid. Packets that can't be tracked d/t error
	ExpectNew     uint32 `json:"expectNew"`     // Entries added after an expectation was already present
	ExpectCreate  uint32 `json:"expectCreate"`  // Expectations added
	ExpectDelete  uint32 `json:"expectDelete"`  // Expectations deleted
	SearchRestart uint32 `json:"searchRestart"` // Conntrack table lookups restarted due to hashtable resizes
}

func NewConntrackStat(e uint32, s uint32, f uint32, n uint32, inv uint32, ign uint32, del uint32, dlst uint32, ins uint32, insfail uint32, drop uint32, edrop uint32, ie uint32, en uint32, ec uint32, ed uint32, sr uint32) *ConntrackStat {
	return &ConntrackStat{
		Entries:       e,
		Searched:      s,
		Found:         f,
		New:           n,
		Invalid:       inv,
		Ignore:        ign,
		Delete:        del,
		DeleteList:    dlst,
		Insert:        ins,
		InsertFailed:  insfail,
		Drop:          drop,
		EarlyDrop:     edrop,
		IcmpError:     ie,
		ExpectNew:     en,
		ExpectCreate:  ec,
		ExpectDelete:  ed,
		SearchRestart: sr,
	}
}

type ConntrackStatList struct {
	items []*ConntrackStat
}

func NewConntrackStatList() *ConntrackStatList {
	return &ConntrackStatList{
		items: []*ConntrackStat{},
	}
}

func (l *ConntrackStatList) Append(c *ConntrackStat) {
	l.items = append(l.items, c)
}

func (l *ConntrackStatList) Items() []ConntrackStat {
	items := make([]ConntrackStat, len(l.items))
	for i, el := range l.items {
		items[i] = *el
	}
	return items
}

// Summary returns a single-element list with totals from all list items.
func (l *ConntrackStatList) Summary() []ConntrackStat {
	summary := NewConntrackStat(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	for _, cs := range l.items {
		summary.Entries += cs.Entries
		summary.Searched += cs.Searched
		summary.Found += cs.Found
		summary.New += cs.New
		summary.Invalid += cs.Invalid
		summary.Ignore += cs.Ignore
		summary.Delete += cs.Delete
		summary.DeleteList += cs.DeleteList
		summary.Insert += cs.Insert
		summary.InsertFailed += cs.InsertFailed
		summary.Drop += cs.Drop
		summary.EarlyDrop += cs.EarlyDrop
		summary.IcmpError += cs.IcmpError
		summary.ExpectNew += cs.ExpectNew
		summary.ExpectCreate += cs.ExpectCreate
		summary.ExpectDelete += cs.ExpectDelete
		summary.SearchRestart += cs.SearchRestart
	}
	return []ConntrackStat{*summary}
}

func (n IOCountersStat) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func (n ConnectionStat) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func (n ProtoCountersStat) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func (a Addr) String() string {
	s, _ := json.Marshal(a)
	return string(s)
}

func (n InterfaceStat) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func (l InterfaceStatList) String() string {
	s, _ := json.Marshal(l)
	return string(s)
}

func (n InterfaceAddr) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func (n ConntrackStat) String() string {
	s, _ := json.Marshal(n)
	return string(s)
}

func Interfaces() (InterfaceStatList, error) {
	return InterfacesWithContext(context.Background())
}

func InterfacesWithContext(ctx context.Context) (InterfaceStatList, error) {
	is, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ret := make(InterfaceStatList, 0, len(is))
	for _, ifi := range is {

		var flags []string
		if ifi.Flags&net.FlagUp != 0 {
			flags = append(flags, "up")
		}
		if ifi.Flags&net.FlagBroadcast != 0 {
			flags = append(flags, "broadcast")
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			flags = append(flags, "loopback")
		}
		if ifi.Flags&net.FlagPointToPoint != 0 {
			flags = append(flags, "pointtopoint")
		}
		if ifi.Flags&net.FlagMulticast != 0 {
			flags = append(flags, "multicast")
		}

		r := InterfaceStat{
			Index:        ifi.Index,
			Name:         ifi.Name,
			MTU:          ifi.MTU,
			HardwareAddr: ifi.HardwareAddr.String(),
			Flags:        flags,
		}
		addrs, err := ifi.Addrs()
		if err == nil {
			r.Addrs = make(InterfaceAddrList, 0, len(addrs))
			for _, addr := range addrs {
				r.Addrs = append(r.Addrs, InterfaceAddr{
					Addr: addr.String(),
				})
			}

		}
		ret = append(ret, r)
	}

	return ret, nil
}

func getIOCountersAll(n []IOCountersStat) ([]IOCountersStat, error) {
	r := IOCountersStat{
		Name: "all",
	}
	for _, nic := range n {
		r.BytesRecv += nic.BytesRecv
		r.PacketsRecv += nic.PacketsRecv
		r.Errin += nic.Errin
		r.Dropin += nic.Dropin
		r.BytesSent += nic.BytesSent
		r.PacketsSent += nic.PacketsSent
		r.Errout += nic.Errout
		r.Dropout += nic.Dropout
	}

	return []IOCountersStat{r}, nil
}
