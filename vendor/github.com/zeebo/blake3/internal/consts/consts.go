package consts

var IV = [...]uint32{IV0, IV1, IV2, IV3, IV4, IV5, IV6, IV7}

const (
	IV0 = 0x6A09E667
	IV1 = 0xBB67AE85
	IV2 = 0x3C6EF372
	IV3 = 0xA54FF53A
	IV4 = 0x510E527F
	IV5 = 0x9B05688C
	IV6 = 0x1F83D9AB
	IV7 = 0x5BE0CD19
)

const (
	Flag_ChunkStart        uint32 = 1 << 0
	Flag_ChunkEnd          uint32 = 1 << 1
	Flag_Parent            uint32 = 1 << 2
	Flag_Root              uint32 = 1 << 3
	Flag_Keyed             uint32 = 1 << 4
	Flag_DeriveKeyContext  uint32 = 1 << 5
	Flag_DeriveKeyMaterial uint32 = 1 << 6
)

const (
	BlockLen = 64
	ChunkLen = 1024
)
