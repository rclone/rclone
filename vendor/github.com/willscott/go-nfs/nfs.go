package nfs

import (
	"context"
)

const (
	nfsServiceID = 100003
)

func init() {
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureNull), onNull)               // 0
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureGetAttr), onGetAttr)         // 1
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureSetAttr), onSetAttr)         // 2
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureLookup), onLookup)           // 3
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureAccess), onAccess)           // 4
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureReadlink), onReadLink)       // 5
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureRead), onRead)               // 6
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureWrite), onWrite)             // 7
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureCreate), onCreate)           // 8
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureMkDir), onMkdir)             // 9
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureSymlink), onSymlink)         // 10
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureMkNod), onMknod)             // 11
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureRemove), onRemove)           // 12
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureRmDir), onRmDir)             // 13
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureRename), onRename)           // 14
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureLink), onLink)               // 15
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureReadDir), onReadDir)         // 16
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureReadDirPlus), onReadDirPlus) // 17
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureFSStat), onFSStat)           // 18
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureFSInfo), onFSInfo)           // 19
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedurePathConf), onPathConf)       // 20
	_ = RegisterMessageHandler(nfsServiceID, uint32(NFSProcedureCommit), onCommit)           // 21
}

func onNull(ctx context.Context, w *response, userHandle Handler) error {
	return w.Write([]byte{})
}
