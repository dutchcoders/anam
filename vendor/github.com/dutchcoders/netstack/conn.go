package netstack

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	ipv4 "github.com/dutchcoders/netstack/ipv4"
	tcp "github.com/dutchcoders/netstack/tcp"
)

type Connection struct {
	closed  bool
	closing bool

	Connected chan bool

	Src, Dst                    net.IP
	SourcePort, DestinationPort uint16

	current *State

	Recv  chan []byte
	Stack *Stack
	// state buffer
	// []State -> this is a backlog with not ack'ed states, in no specific order, all incoming will be added to
	// this buffer
	buffer []byte
}

// Read reads data from the connection.
// Read can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (conn *Connection) Read(b []byte) (n int, err error) {
	// clear out current buffer
	if len(conn.buffer) > 0 {
		n := copy(b, conn.buffer[:])
		conn.buffer = conn.buffer[n:]
		return n, nil
	}

	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Timeout occured")
		return 0, errors.New("Timeout occured.")
	case _, ok := <-conn.Recv:
		if !ok {
			// connection closed?
			return 0, io.EOF
		}

		n := copy(b[:], conn.buffer[:])
		conn.buffer = conn.buffer[n:]
		return n, nil
	}
}

// Write writes data to the connection.
// Write can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetWriteDeadline.
func (c *Connection) Write(b []byte) (n int, err error) {
	if c.closing || c.closed {
		return 0, errors.New("Writing to a closed connection.")
	}

	state := c.current

	state.Lock()
	defer state.Unlock()

	iph := ipv4.Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 52 + len(b),
		Flags:    2,
		TTL:      128,
		Protocol: 6,
		Src:      c.Src,
		Dst:      c.Dst,
		Options:  []byte{},
		ID:       state.ID,
	}

	th := tcp.Header{
		Source:      c.SourcePort,
		Destination: c.DestinationPort,
		SeqNum:      state.SendNext,
		AckNum:      state.RecvNext,
		DataOffset:  5,
		Reserved:    0,
		ECN:         0,
		Ctrl:        tcp.PSH | tcp.ACK,
		Window:      64420,
		Checksum:    0,
		Urgent:      0,
		Options:     []tcp.Option{},
		Payload:     b,
	}

	if data, err := th.Marshal(); err == nil {
		iph.Payload = data
	} else {
		return 0, err
	}

	if data, err := iph.Marshal(); err == nil {
		c.Stack.send(data)
	} else {
		return 0, err
	}

	state.ID++
	state.SendNext += uint32(len(b))

	return len(b), nil
}

// LocalAddr returns the local network address.
func (c *Connection) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   c.Src,
		Port: int(c.SourcePort),
		Zone: "",
	}
}

// RemoteAddr returns the remote network address.
func (c *Connection) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   c.Dst,
		Port: int(c.DestinationPort),
		Zone: "",
	}
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail with a timeout (see type Error) instead of
// blocking. The deadline applies to all future I/O, not just
// the immediately following call to Read or Write.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Connection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets the deadline for future Read calls.
// A zero value for t means Read will not time out.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *Connection) close() {
	if c.closed {
		return
	}

	c.closed = true
	close(c.Recv)
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *Connection) Close() error {
	// return nil
	state := c.current

	state.Lock()
	defer state.Unlock()

	if c.closing || c.closed {
		// already closed
		return nil
	}

	c.closing = true

	iph := ipv4.Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 52,
		Flags:    2,
		TTL:      128,
		Protocol: 6,
		Src:      c.Src,
		Dst:      c.Dst,
		Options:  []byte{},
		ID:       state.ID,
	}

	th := tcp.Header{
		Source:      c.SourcePort,
		Destination: c.DestinationPort,
		SeqNum:      state.SendNext,
		AckNum:      state.RecvNext,
		DataOffset:  5,
		Reserved:    0,
		ECN:         0,
		Ctrl:        tcp.FIN | tcp.ACK,
		Window:      64420,
		Checksum:    0,
		Urgent:      0,
		Options:     []tcp.Option{},
		Payload:     []byte{},
	}

	c.current.SocketState = SocketFinWait1

	if data, err := th.Marshal(); err != nil {
		return err
	} else {
		iph.Payload = data
	}

	if data, err := iph.Marshal(); err != nil {
		return err
	} else {
		c.Stack.send(data)
	}

	state.ID++
	state.SendNext++

	// wait for close to be ack'ed

	// send fin

	// defer close(state.Conn.Recv)
	return nil
}

func (c *Connection) Receive() chan []byte {
	return c.Recv
}

func (c *Connection) Open(src net.IP, dst net.IP, port int) error {
	c.Src = src
	c.Dst = dst
	c.closed = false

	// this should be verified if it is free
	c.SourcePort = 1000 + uint16(c.Stack.r.Intn(32768))
	c.DestinationPort = uint16(port)

	// prevent running
	sendNext := uint32(c.Stack.r.Intn(2147483648))

	id := int(c.Stack.r.Uint32() % 65535)

	state := &State{
		SrcPort:  c.SourcePort,
		DestPort: c.DestinationPort,

		SrcIP:  src,
		DestIP: dst,

		Last: time.Now(),
		ID:   id,

		RecvNext: 0,
		SendNext: sendNext,

		Conn: c,
	}

	state.SocketState = SocketSynSent

	iph := ipv4.Header{
		Version:  4,
		Len:      20,
		TOS:      0,
		TotalLen: 52,
		Flags:    2,
		TTL:      128,
		Protocol: 6,
		Src:      src,
		Dst:      dst,
		Options:  []byte{},
		ID:       id,
	}

	th := tcp.Header{
		Source:      c.SourcePort,
		Destination: c.DestinationPort,
		SeqNum:      sendNext,
		AckNum:      0,
		DataOffset:  5,
		Reserved:    0,
		ECN:         0,
		Ctrl:        tcp.SYN,
		Window:      64420,
		Checksum:    0,
		Urgent:      0,
		Options:     []tcp.Option{},
		Payload:     []byte{},
	}

	// TCPOptions:
	// Maximum segment size: 1460 bytes
	// Timestamps:
	// nop
	// Window scale

	if data, err := th.Marshal(); err == nil {
		iph.Payload = data
	} else {
		return err
	}

	if data, err := iph.Marshal(); err == nil {
		c.Stack.send(data)
	} else {
		return err
	}

	state.ID++
	state.SendNext = state.SendNext + 1

	c.current = state

	stateTable = append(stateTable, state)

	return nil
}
