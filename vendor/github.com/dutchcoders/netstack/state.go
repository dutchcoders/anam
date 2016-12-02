package netstack

import (
	"net"
	"sync"
	"time"
)

type State struct {
	sync.Mutex

	SrcIP    net.IP
	SrcPort  uint16
	DestIP   net.IP
	DestPort uint16

	Last time.Time

	RecvNext           uint32
	SendNext           uint32
	SendUnAcknowledged uint32
	LastAcked          uint32

	SocketState SocketState

	ID int

	Conn *Connection
}
