// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

// SplitN splits the marshaled form of the Packet into a number of
// frames such that each frame is at most n bytes. It calls
// the callback with every such frame. If n is zero, a reasonable
// default is used.
func SplitN(pkt Packet, n int, cb func(fr Frame) error) error {
	for {
		fr := Frame{
			Data:    pkt.Data,
			ID:      pkt.ID,
			Kind:    pkt.Kind,
			Control: pkt.Control,
			Done:    true,
		}

		fr.Data, pkt.Data = SplitData(pkt.Data, n)
		fr.Done = len(pkt.Data) == 0

		if err := cb(fr); err != nil {
			return err
		}
		if fr.Done {
			return nil
		}
	}
}

// SplitData is used to split a buffer if it is larger than n bytes.
// If n is zero, a reasonable default is used. If n is less than zero
// then it does not split.
func SplitData(buf []byte, n int) (prefix, suffix []byte) {
	switch {
	case n == 0:
		n = 64 * 1024
	case n < 0:
		n = 0
	}

	if len(buf) > n && n > 0 {
		return buf[:n], buf[n:]
	}
	return buf, nil
}
