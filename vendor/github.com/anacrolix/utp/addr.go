package utp

import "net"

type Addr struct {
	Socket net.Addr
}

func (me Addr) Network() string {
	return "utp/" + me.Socket.Network()
}

func (me Addr) String() string {
	return me.Socket.String()
}
