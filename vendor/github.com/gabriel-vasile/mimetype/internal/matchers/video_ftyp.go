package matchers

var (
	mp4Sigs = []sig{
		ftypSig("avc1"), ftypSig("dash"), ftypSig("iso2"), ftypSig("iso3"),
		ftypSig("iso4"), ftypSig("iso5"), ftypSig("iso6"), ftypSig("isom"),
		ftypSig("mmp4"), ftypSig("mp41"), ftypSig("mp42"), ftypSig("mp4v"),
		ftypSig("mp71"), ftypSig("MSNV"), ftypSig("NDAS"), ftypSig("NDSC"),
		ftypSig("NSDC"), ftypSig("NSDH"), ftypSig("NDSM"), ftypSig("NDSP"),
		ftypSig("NDSS"), ftypSig("NDXC"), ftypSig("NDXH"), ftypSig("NDXM"),
		ftypSig("NDXP"), ftypSig("NDXS"), ftypSig("F4V "), ftypSig("F4P "),
	}
	threeGPSigs = []sig{
		ftypSig("3gp1"), ftypSig("3gp2"), ftypSig("3gp3"), ftypSig("3gp4"),
		ftypSig("3gp5"), ftypSig("3gp6"), ftypSig("3gp7"), ftypSig("3gs7"),
		ftypSig("3ge6"), ftypSig("3ge7"), ftypSig("3gg6"),
	}
	threeG2Sigs = []sig{
		ftypSig("3g24"), ftypSig("3g25"), ftypSig("3g26"), ftypSig("3g2a"),
		ftypSig("3g2b"), ftypSig("3g2c"), ftypSig("KDDI"),
	}
	amp4Sigs = []sig{
		// audio for Adobe Flash Player 9+
		ftypSig("F4A "), ftypSig("F4B "),
		// Apple iTunes AAC-LC (.M4A) Audio
		ftypSig("M4B "), ftypSig("M4P "),
		// MPEG-4 (.MP4) for SonyPSP
		ftypSig("MSNV"),
		// Nero Digital AAC Audio
		ftypSig("NDAS"),
	}
	qtSigs  = []sig{ftypSig("qt  "), ftypSig("moov")}
	mqvSigs = []sig{ftypSig("mqt ")}
	m4aSigs = []sig{ftypSig("M4A ")}
	// TODO: add support for remaining video formats at ftyps.com.
)

// Mp4 matches an MP4 file.
func Mp4(in []byte) bool {
	return detect(in, mp4Sigs)
}

// ThreeGP matches a 3GPP file.
func ThreeGP(in []byte) bool {
	return detect(in, threeGPSigs)
}

// ThreeG2 matches a 3GPP2 file.
func ThreeG2(in []byte) bool {
	return detect(in, threeG2Sigs)
}

// AMp4 matches an audio MP4 file.
func AMp4(in []byte) bool {
	return detect(in, amp4Sigs)
}

// QuickTime matches a QuickTime File Format file.
func QuickTime(in []byte) bool {
	return detect(in, qtSigs)
}

// Mqv matches a Sony / Mobile QuickTime  file.
func Mqv(in []byte) bool {
	return detect(in, mqvSigs)
}

// M4a matches an audio M4A file.
func M4a(in []byte) bool {
	return detect(in, m4aSigs)
}
