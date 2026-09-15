package smb

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

// closeResponseBody is the SMB2 CLOSE response ([MS-SMB2] 2.2.16):
// StructureSize 60, with all attribute fields zeroed (we never request
// post-query attributes).
func closeResponseBody() []byte {
	b := make([]byte, 60)
	le.PutUint16(b[0:2], 60)
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
