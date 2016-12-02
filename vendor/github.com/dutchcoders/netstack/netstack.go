package netstack

// +build amd64,linux

import (
	"fmt"
	"log"
	"math/rand"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

const (
	MaxEpollEvents    = 64
	DefaultBufferSize = 65535
)

type SocketState int

const (
	SocketClosed SocketState = iota
	SocketListen
	SocketSynReceived
	SocketSynSent
	SocketEstablished
	SocketFinWait1
	SocketFinWait2
	SocketClosing
	SocketTimeWait
	SocketCloseWait
	SocketLastAck
)

func (ss SocketState) String() string {
	switch ss {
	case SocketClosed:
		return "SocketClosed"
	case SocketListen:
		return "SocketListen"
	case SocketSynReceived:
		return "SocketSynReceived"
	case SocketSynSent:
		return "SocketSynSent"
	case SocketEstablished:
		return "SocketEstablished"
	case SocketFinWait1:
		return "SocketFinWait1"
	case SocketFinWait2:
		return "SocketFinWait2"
	case SocketClosing:
		return "SocketClosing"
	case SocketTimeWait:
		return "SocketTimeWait"
	case SocketCloseWait:
		return "SocketCloseWait"
	case SocketLastAck:
		return "SocketLastAck"
	default:
		return fmt.Sprintf("Unknown state: %d", int(ss))
	}
}

func to4byte(addr string) [4]byte {
	parts := strings.Split(addr, ".")
	b0, err := strconv.Atoi(parts[0])
	fmt.Println(addr)
	if err != nil {
		log.Fatalf("to4byte: %s (latency works with IPv4 addresses only, but not IPv6!)\n", err)
	}
	b1, _ := strconv.Atoi(parts[1])
	b2, _ := strconv.Atoi(parts[2])
	b3, _ := strconv.Atoi(parts[3])
	return [4]byte{byte(b0), byte(b1), byte(b2), byte(b3)}
}

type socket struct {
}

type listener struct {
	s chan bool
}

func (l *listener) Accept() (socket, error) {
	<-l.s

	// wait for packets to arrive. Return a socket
	return socket{}, nil
}
