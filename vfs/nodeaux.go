package vfs

import "sync/atomic"

// auxEntry associates an owner with a value attached to a node.
type auxEntry struct {
	owner, value any
}

// aux holds auxiliary values attached to a node, keyed by owner.
//
// It is embedded in Dir and File to provide the Aux, SetAux, Sys and
// SetSys methods of the Node interface.
//
// Reads are lock free. Writes copy the entry list, which is assumed
// to be very short.
type aux struct {
	entries atomic.Pointer[[]auxEntry]
}

// sysOwner is the owner Sys and SetSys attach their value under.
type sysOwner struct{}

// Aux returns the value attached to the node for owner, or nil if
// none is attached.
func (a *aux) Aux(owner any) any {
	if entries := a.entries.Load(); entries != nil {
		for _, entry := range *entries {
			if entry.owner == owner {
				return entry.value
			}
		}
	}
	return nil
}

// SetAux attaches value to the node for owner, replacing any value
// owner attached before. Attaching nil removes owner's value.
//
// Values attached by different owners are independent, so multiple
// users of a shared VFS do not conflict. Owner must be comparable and
// unique to the user - a pointer to the user's filesystem struct is a
// good choice.
func (a *aux) SetAux(owner, value any) {
	for {
		old := a.entries.Load()
		var entries []auxEntry
		if old != nil {
			entries = make([]auxEntry, 0, len(*old)+1)
			for _, entry := range *old {
				if entry.owner != owner {
					entries = append(entries, entry)
				}
			}
		}
		if value != nil {
			entries = append(entries, auxEntry{owner: owner, value: value})
		}
		var next *[]auxEntry
		if len(entries) != 0 {
			next = &entries
		}
		if a.entries.CompareAndSwap(old, next) {
			return
		}
	}
}

// Sys returns the value set with SetSys (can be nil) - satisfies
// os.FileInfo.
//
// This is what callers reading the node through os.FileInfo (e.g. the
// NFS server library) will see. It is reserved for users which need
// to control that; users caching their own data on the node should
// use Aux and SetAux instead. As the VFS may be shared, the value
// stored here should be derived from the node alone so that all users
// store the same value.
func (a *aux) Sys() any {
	return a.Aux(sysOwner{})
}

// SetSys sets the value returned by Sys - see Sys for the contract.
func (a *aux) SetSys(x any) {
	a.SetAux(sysOwner{}, x)
}
