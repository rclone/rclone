// oauthutil parts go1.8+

//+build go1.8

package oauthutil

import "github.com/ncw/rclone/fs"

func (s *authServer) Stop() {
	fs.Debugf(nil, "Closing auth server")
	if s.code != nil {
		close(s.code)
		s.code = nil
	}
	_ = s.listener.Close()

	// close the server
	_ = s.server.Close()
}
