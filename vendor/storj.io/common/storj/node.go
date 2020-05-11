// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto/sha256"
	"crypto/x509/pkix"
	"database/sql/driver"
	"encoding/json"
	"math/bits"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"

	"storj.io/common/peertls/extensions"
)

var (
	// ErrNodeID is used when something goes wrong with a node id.
	ErrNodeID = errs.Class("node ID error")
	// ErrVersion is used for identity version related errors.
	ErrVersion = errs.Class("node ID version error")
)

// NodeIDSize is the byte length of a NodeID
const NodeIDSize = sha256.Size

// NodeID is a unique node identifier
type NodeID [NodeIDSize]byte

// NodeIDList is a slice of NodeIDs (implements sort)
type NodeIDList []NodeID

// NewVersionedID adds an identity version to a node ID.
func NewVersionedID(id NodeID, version IDVersion) NodeID {
	var versionedID NodeID
	copy(versionedID[:], id[:])

	versionedID[NodeIDSize-1] = byte(version.Number)
	return versionedID
}

// NewVersionExt creates a new identity version certificate extension for the
// given identity version,
func NewVersionExt(version IDVersion) pkix.Extension {
	return pkix.Extension{
		Id:    extensions.IdentityVersionExtID,
		Value: []byte{byte(version.Number)},
	}
}

// NodeIDFromString decodes a base58check encoded node id string
func NodeIDFromString(s string) (NodeID, error) {
	idBytes, versionNumber, err := base58.CheckDecode(s)
	if err != nil {
		return NodeID{}, ErrNodeID.Wrap(err)
	}
	unversionedID, err := NodeIDFromBytes(idBytes)
	if err != nil {
		return NodeID{}, err
	}

	version := IDVersions[IDVersionNumber(versionNumber)]
	return NewVersionedID(unversionedID, version), nil
}

// NodeIDsFromBytes converts a 2d byte slice into a list of nodes
func NodeIDsFromBytes(b [][]byte) (ids NodeIDList, err error) {
	var idErrs []error
	for _, idBytes := range b {
		id, err := NodeIDFromBytes(idBytes)
		if err != nil {
			idErrs = append(idErrs, err)
			continue
		}

		ids = append(ids, id)
	}

	if err = errs.Combine(idErrs...); err != nil {
		return nil, err
	}
	return ids, nil
}

// NodeIDFromBytes converts a byte slice into a node id
func NodeIDFromBytes(b []byte) (NodeID, error) {
	bLen := len(b)
	if bLen != len(NodeID{}) {
		return NodeID{}, ErrNodeID.New("not enough bytes to make a node id; have %d, need %d", bLen, len(NodeID{}))
	}

	var id NodeID
	copy(id[:], b)
	return id, nil
}

// String returns NodeID as base58 encoded string with checksum and version bytes
func (id NodeID) String() string {
	unversionedID := id.unversioned()
	return base58.CheckEncode(unversionedID[:], byte(id.Version().Number))
}

// IsZero returns whether NodeID is unassigned
func (id NodeID) IsZero() bool {
	return id == NodeID{}
}

// Bytes returns raw bytes of the id
func (id NodeID) Bytes() []byte { return id[:] }

// Less returns whether id is smaller than other in lexicographic order.
func (id NodeID) Less(other NodeID) bool {
	for k, v := range id {
		if v < other[k] {
			return true
		} else if v > other[k] {
			return false
		}
	}
	return false
}

// Version returns the version of the identity format
func (id NodeID) Version() IDVersion {
	versionNumber := id.versionByte()
	if versionNumber == 0 {
		return IDVersions[V0]
	}

	version, err := GetIDVersion(IDVersionNumber(versionNumber))
	// NB: when in doubt, use V0
	if err != nil {
		return IDVersions[V0]
	}

	return version
}

// Difficulty returns the number of trailing zero bits in a node ID
func (id NodeID) Difficulty() (uint16, error) {
	idLen := len(id)
	var b byte
	var zeroBits int
	// NB: last difficulty byte is used for version
	for i := 2; i <= idLen; i++ {
		b = id[idLen-i]

		if b != 0 {
			zeroBits = bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				// we already checked that b != 0.
				return 0, ErrNodeID.New("impossible codepath!")
			}

			return uint16((i-1)*8 + zeroBits), nil
		}
	}

	return 0, ErrNodeID.New("difficulty matches id hash length: %d; hash (hex): % x", idLen, id)
}

// Marshal serializes a node id
func (id NodeID) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a node ID into the passed byte slice
func (id *NodeID) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

// Unmarshal deserializes a node ID
func (id *NodeID) Unmarshal(data []byte) error {
	var err error
	*id, err = NodeIDFromBytes(data)
	return err
}

func (id NodeID) versionByte() byte {
	return id[NodeIDSize-1]
}

// unversioned returns the node ID with the version byte replaced with `0`.
// NB: Legacy node IDs (i.e. pre-identity-versions) with a difficulty less
// than `8` are unsupported.
func (id NodeID) unversioned() NodeID {
	unversionedID := NodeID{}
	copy(unversionedID[:], id[:NodeIDSize-1])
	return unversionedID
}

// Size returns the length of a node ID (implements gogo's custom type interface)
func (id *NodeID) Size() int {
	return len(id)
}

// MarshalJSON serializes a node ID to a json string as bytes
func (id NodeID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// Value converts a NodeID to a database field
func (id NodeID) Value() (driver.Value, error) {
	return id.Bytes(), nil
}

// Scan extracts a NodeID from a database field
func (id *NodeID) Scan(src interface{}) (err error) {
	b, ok := src.([]byte)
	if !ok {
		return ErrNodeID.New("NodeID Scan expects []byte")
	}
	n, err := NodeIDFromBytes(b)
	*id = n
	return err
}

// UnmarshalJSON deserializes a json string (as bytes) to a node ID
func (id *NodeID) UnmarshalJSON(data []byte) error {
	var unquoted string
	err := json.Unmarshal(data, &unquoted)
	if err != nil {
		return err
	}

	*id, err = NodeIDFromString(unquoted)
	if err != nil {
		return err
	}
	return nil
}

// Strings returns a string slice of the node IDs
func (n NodeIDList) Strings() []string {
	var strings []string
	for _, nid := range n {
		strings = append(strings, nid.String())
	}
	return strings
}

// Bytes returns a 2d byte slice of the node IDs
func (n NodeIDList) Bytes() (idsBytes [][]byte) {
	for _, nid := range n {
		idsBytes = append(idsBytes, nid.Bytes())
	}
	return idsBytes
}

// Len implements sort.Interface.Len()
func (n NodeIDList) Len() int { return len(n) }

// Swap implements sort.Interface.Swap()
func (n NodeIDList) Swap(i, j int) { n[i], n[j] = n[j], n[i] }

// Less implements sort.Interface.Less()
func (n NodeIDList) Less(i, j int) bool { return n[i].Less(n[j]) }
