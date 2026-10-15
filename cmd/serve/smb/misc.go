package smb

import "github.com/rclone/rclone/vfs"

// Small fixed-size response bodies for the simple commands.

// echoResponseBody is the SMB2 ECHO response ([MS-SMB2] 2.2.29): StructureSize 4.
func echoResponseBody() []byte {
	b := make([]byte, 4)
	le.PutUint16(b[0:2], 4)
	return b
}

// treeDisconnectResponseBody is the SMB2 TREE_DISCONNECT response: StructureSize 4.
func treeDisconnectResponseBody() []byte {
	b := make([]byte, 4)
	le.PutUint16(b[0:2], 4)
	return b
}

// logoffResponseBody is the SMB2 LOGOFF response: StructureSize 4.
func logoffResponseBody() []byte {
	b := make([]byte, 4)
	le.PutUint16(b[0:2], 4)
	return b
}

// closeFlagPostQueryAttrib is SMB2_CLOSE_FLAG_POSTQUERY_ATTRIB: the client wants
// the file's attributes in the CLOSE response.
const closeFlagPostQueryAttrib uint16 = 0x0001

// closeResponseBody is the SMB2 CLOSE response ([MS-SMB2] 2.2.16): StructureSize
// 60. It carries the attributes of node if that is not nil, and zeros otherwise.
func closeResponseBody(node vfs.Node) []byte {
	b := make([]byte, 60)
	le.PutUint16(b[0:2], 60)
	if node != nil {
		attrs, size, mtime := nodeAttrs(node)
		ft := timeToFiletime(mtime)
		le.PutUint16(b[2:4], closeFlagPostQueryAttrib)
		le.PutUint64(b[8:16], ft)  // CreationTime
		le.PutUint64(b[16:24], ft) // LastAccessTime
		le.PutUint64(b[24:32], ft) // LastWriteTime
		le.PutUint64(b[32:40], ft) // ChangeTime
		le.PutUint64(b[40:48], uint64(size))
		le.PutUint64(b[48:56], uint64(size))
		le.PutUint32(b[56:60], attrs)
	}
	return b
}

// lockResponseBody is the SMB2 LOCK response ([MS-SMB2] 2.2.27): StructureSize
// 4. We do not enforce byte-range locks but grant every request so clients
// (notably Windows) that rely on advisory locking can proceed.
func lockResponseBody() []byte {
	b := make([]byte, 4)
	le.PutUint16(b[0:2], 4)
	return b
}
