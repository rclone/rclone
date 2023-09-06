package noiseconn

import (
	"net"

	"github.com/flynn/noise"
)

type Listener struct {
	net.Listener
	config noise.Config
}

var _ net.Listener = (*Listener)(nil)

func NewListener(inner net.Listener, config noise.Config) *Listener {
	return &Listener{
		Listener: inner,
		config:   config,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewConn(conn, l.config)
}
