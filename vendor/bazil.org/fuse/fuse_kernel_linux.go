package fuse

func openFlags(flags uint32) OpenFlags {
	// on amd64, the 32-bit O_LARGEFILE flag is always seen;
	// on i386, the flag probably depends on the app
	// requesting, but in any case should be utterly
	// uninteresting to us here; our kernel protocol messages
	// are not directly related to the client app's kernel
	// API/ABI
	flags &^= 0x8000

	return OpenFlags(flags)
}
