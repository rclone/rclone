// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"net"

	"storj.io/common/storj"
)

var (
	knownNodeIDs = map[string]storj.NodeID{}
)

func init() {
	// !!!! NOTE !!!!
	//
	// These exist for backwards compatibility.
	//
	// Do not add more here, any new satellite MUST use node ID,
	// Adding new satellites here will break forwards compatibility.
	for _, nodeURL := range []string{
		"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777",
		"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777",
		"121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777",
		"121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777",
		"12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777",
		"12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777",
		"118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW@satellite.stefan-benten.de:7777",
		"1wFTAgs9DP5RSnCqKV1eLf6N9wtk4EAtmN5DpSxcs8EjT69tGE@saltlake.tardigrade.io:7777",
	} {
		url, err := storj.ParseNodeURL(nodeURL)
		if err != nil {
			panic(err)
		}
		knownNodeIDs[url.Address] = url.ID
		host, _, err := net.SplitHostPort(url.Address)
		if err != nil {
			panic(err)
		}
		knownNodeIDs[host] = url.ID
	}
}

// KnownNodeID looks for a well-known node id for a given address
func KnownNodeID(address string) (id storj.NodeID, known bool) {
	id, known = knownNodeIDs[address]
	if !known {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return id, false
		}
		id, known = knownNodeIDs[host]
	}
	return id, known
}
