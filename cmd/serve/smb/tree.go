package smb

import "strings"

// handleTreeConnect handles an SMB2 TREE_CONNECT request. We expose a single
// share (named by --name), so we accept a connection to that share and reject
// everything else with STATUS_BAD_NETWORK_NAME.
func (c *conn) handleTreeConnect(h header, body []byte) (status uint32, treeID uint32, respBody []byte) {
	if len(body) < 8 {
		return statusInvalidParameter, 0, errorResponseBody()
	}
	path := ""
	if buf := bufferAt(body, le.Uint16(body[4:6]), le.Uint16(body[6:8])); buf != nil {
		path = utf16leToString(buf)
	}
	share := shareFromPath(path)

	// Accept our disk share and the special IPC$ share. cifs (Linux) connects
	// to IPC$ for DFS referral resolution and refuses to mount without it.
	var shareType byte
	switch {
	case strings.EqualFold(share, c.server.shareName):
		shareType = shareTypeDisk
	case strings.EqualFold(share, "IPC$"):
		shareType = shareTypePipe
	default:
		return statusBadNetworkName, 0, errorResponseBody()
	}

	treeID = c.server.nextTreeID()
	if shareType == shareTypePipe {
		// IPC$: remember the tree so file opens on it are rejected rather than
		// resolved as VFS paths (a real file named e.g. srvsvc must not be opened).
		c.pipeTrees[treeID] = struct{}{}
	}
	rb := make([]byte, 16)
	le.PutUint16(rb[0:2], 16)           // StructureSize
	rb[2] = shareType                   // ShareType
	rb[3] = 0                           // Reserved
	le.PutUint32(rb[4:8], 0)            // ShareFlags
	le.PutUint32(rb[8:12], 0)           // Capabilities
	le.PutUint32(rb[12:16], 0x001F01FF) // MaximalAccess (full)
	return statusSuccess, treeID, rb
}

// shareFromPath extracts the share name from a UNC path like
// `\\host\share` (the final path component).
func shareFromPath(p string) string {
	p = strings.TrimPrefix(p, `\\`)
	if i := strings.LastIndex(p, `\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
