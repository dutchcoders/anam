package netstack

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"syscall"
	"time"

	ipv4 "github.com/dutchcoders/netstack/ipv4"
	tcp "github.com/dutchcoders/netstack/tcp"
)

func htons(n uint16) uint16 {
	var (
		high uint16 = n >> 8
		ret  uint16 = n<<8 + high
	)
	return ret
}

type Stack struct {
	fd   int
	epfd int
	r    *rand.Rand

	m sync.Mutex

	sendQueue [][]byte
	buffer    []byte

	src net.IP

	networkInterface *net.Interface
}

var ErrNoState = errors.New("No state for packet.")

func New(intf string) (*Stack, error) {
	if networkInterface, err := net.InterfaceByName(intf); err != nil {
		return nil, fmt.Errorf("The selected network interface %s does not exist.\n", intf)
	} else if fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP); err != nil {
		return nil, fmt.Errorf("Could not create socket: %s", err.Error())
	} else if fd < 0 {
		return nil, fmt.Errorf("Socket error: return < 0")
	} else if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		return nil, err
	} else if epfd, err := syscall.EpollCreate1(0); err != nil {
		return nil, fmt.Errorf("epoll_create1: %s", err.Error())
	} else if err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLERR, /*| syscall.EPOLL_NONBLOCK  | syscall.EPOLLOUT | syscall.EPOLLET*/
		Fd:     int32(fd),
	}); err != nil {
		return nil, fmt.Errorf("epollctl: %s", err.Error())
	} else if addrs, err := networkInterface.Addrs(); err != nil {
		return nil, fmt.Errorf("Could not retrieve ip addrs: %s", err.Error())
	} else {
		r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

		return &Stack{
			fd:               fd,
			epfd:             epfd,
			r:                r,
			src:              addrs[0].(*net.IPNet).IP,
			networkInterface: networkInterface,
		}, nil
	}
}

func (s *Stack) Connect(dest net.IP, port int) (*Connection, error) {
	conn := &Connection{
		Connected: make(chan bool, 1),
		Stack:     s,
		Recv:      make(chan []byte),
		Src:       s.src,
		Dst:       dest,
	}

	if err := conn.Open(s.src, dest, port); err != nil {
		return nil, err
	}

	// wait for ack?

	select {
	case <-time.After(30 * time.Second):
		return nil, errors.New("Timeout occured.")
	case <-conn.Connected:
		return conn, nil
	}
}

func (s *Stack) Close() {
	syscall.Close(s.epfd)
	syscall.Close(s.fd)
}

func (s Stack) Listen() (*listener, error) {
	return &listener{
		s: make(chan bool),
	}, nil
}

var buffer = make([]byte, DefaultBufferSize)

func (s *Stack) Start() error {
	go func() {
		var events [MaxEpollEvents]syscall.EpollEvent

		for {
			nevents, err := syscall.EpollWait(s.epfd, events[:], -1)
			if err != nil {
				fmt.Println("epoll_wait: ", err)
				break
			}

			for ev := 0; ev < nevents; ev++ {
				if events[ev].Events&syscall.EPOLLERR == syscall.EPOLLERR {
					s.handleEventPollErr(events[ev])
				}

				if events[ev].Events&syscall.EPOLLIN == syscall.EPOLLIN {
					s.handleEventPollIn(events[ev])
				}
			}
		}

	}()

	return nil
}

func (s *Stack) handleEventPollIn(event syscall.EpollEvent) {
	if n, _, err := syscall.Recvfrom(int(event.Fd), buffer, 0); err != nil {
		fmt.Println("Could not receive from descriptor: %s", err.Error())
		return
	} else if n == 0 {
		// no packets received
		return
	} else if iph, err := ipv4.Parse(buffer); err != nil {
		fmt.Println(fmt.Errorf("Error parsing ip header: ", err.Error()))
	} else if iph.Len < 5 {
		fmt.Println(fmt.Errorf("IP header length is invalid."))
	} else {
		data := buffer[20:n]

		switch iph.Protocol {
		case 6 /* tcp */ :
			if err := s.handleTCP(iph, data); err == ErrNoState {
			} else if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		case 17 /* udp */ :
			if err := s.handleUDP(iph, data); err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		default:
			fmt.Printf("Unknown protocol: %d\n", iph.Protocol)
		}
	}
}

func (s *Stack) handleEventPollErr(event syscall.EpollEvent) {
	if v, err := syscall.GetsockoptInt(int(event.Fd), syscall.SOL_SOCKET, syscall.SO_ERROR); err != nil {
		fmt.Println("Error", err)
	} else {
		fmt.Println("Error val", v)
	}
}

func (s *Stack) send(data []byte) error {
	// update ip checksum
	csum := uint32(0)

	// calculate correct ip header length here, with opts
	length := 20 // len(data1) - 1
	for i := 0; i < length; i += 2 {
		if i == 10 {
			continue
		}

		csum += uint32(data[i]) << 8
		csum += uint32(data[i+1])
	}

	for {
		// Break when sum is less or equals to 0xFFFF
		if csum <= 65535 {
			break
		}
		// Add carry to the sum
		csum = (csum >> 16) + uint32(uint16(csum))
	}

	csum = uint32(^uint16(csum))

	data[10] = uint8((csum >> 8) & 0xFF)
	data[11] = uint8(csum & 0xFF)

	// update tcp checksum
	length = len(data) - 20

	csum = uint32(0)

	csum += (uint32(data[12]) + uint32(data[14])) << 8
	csum += uint32(data[13]) + uint32(data[15])
	csum += (uint32(data[16]) + uint32(data[18])) << 8
	csum += uint32(data[17]) + uint32(data[19])

	csum += uint32(6)
	csum += uint32(length) & 0xffff
	csum += uint32(length) >> 16

	length = len(data) - 1

	// calculate correct ip header length here.
	for i := 20; i < length; i += 2 {
		if i == 20+16 {
			continue
		}

		csum += uint32(data[i]) << 8
		csum += uint32(data[i+1])
	}

	if len(data)%2 == 1 {
		csum += uint32(data[length]) << 8
	}

	for csum > 0xffff {
		csum = (csum >> 16) + (csum & 0xffff)
	}

	csum = uint32(^uint16(csum + (csum >> 16)))

	data[20+16] = uint8((csum >> 8) & 0xFF)
	data[20+17] = uint8(csum & 0xFF)

	to := &syscall.SockaddrInet4{Port: int(0), Addr: [4]byte{data[16], data[17], data[18], data[19]}} //[4]byte{dest[0], dest[1], dest[2], dest[
	if err := syscall.Sendto((int(s.fd)), data, 0, to); err != nil {
		fmt.Println(fmt.Sprintf("Error: %s %d\n", err.Error(), len(data)))
		return err
	}

	return nil
}

func (s *Stack) handleTCP(iph *ipv4.Header, data []byte) error {
	var th *tcp.Header
	if v, err := tcp.Parse(data); err != nil {
		fmt.Printf("err th: %s\n", err)
		return err
	} else {
		th = &v
	}

	state := stateTable.Get(iph.Src, iph.Dst, th.Source, th.Destination)
	if state != nil {
	} else if th.HasFlag(tcp.SYN) {
		// listening on port
		// send SYN+ACK
		return ErrNoState
	} else {
		// SEND RST?
		return ErrNoState
	}

	state.Lock()
	defer state.Unlock()

	if th.HasFlag(tcp.RST) {
		state.Conn.close()
		state.SocketState = SocketClosed
		return nil
	}

	// should we ignore packets out of sequence?
	// retransmit everything after last ack num
	//  after timeout again
	// first ack
	// if PSH then push, otherwise buffer, for now we're just pushing.
	if state.RecvNext == 0 {
		state.RecvNext = th.SeqNum
	}

	if state.RecvNext != th.SeqNum {
		// fmt.Printf("Unexpected packet: id=%d, seqnum=%d, expected %d (%d)\n%s %s\n", iph.ID, th.SeqNum, state.RecvNext, int(state.RecvNext)-int(th.SeqNum), iph.String(), th.String())
		// we could queue those packets for later usage
		return nil
	}

	// should keep track of ack (sendnext) here as well, to see which of our
	// packets are acked already, and what we need to resend. Currently there
	// is no resend in place
	state.RecvNext += uint32(len(th.Payload))

	if th.HasFlag(tcp.SYN) || th.HasFlag(tcp.FIN) {
		state.RecvNext++
	}

	if state.SocketState == SocketSynSent {
		if !th.HasFlag(tcp.SYN | tcp.ACK) {
			fmt.Printf("StateSynSent: unexpected ctrl %d\n", th.Ctrl)
			state.SocketState = SocketClosed
			return nil
		}

		iph := ipv4.Header{
			Version:  4,
			Len:      20,
			TOS:      0,
			TotalLen: 52,
			Flags:    2,
			TTL:      128,
			Protocol: 6,
			Src:      iph.Dst,
			Dst:      iph.Src,
			Options:  []byte{},
			ID:       state.ID,
		}

		th := tcp.Header{
			Source:      th.Destination,
			Destination: th.Source,
			SeqNum:      state.SendNext,
			AckNum:      state.RecvNext,
			DataOffset:  5,
			Reserved:    0,
			ECN:         0,
			Ctrl:        tcp.ACK,
			Window:      64420,
			Checksum:    0,
			Urgent:      0,
			Options:     []tcp.Option{},
			Payload:     []byte{},
		}

		if data, err := th.Marshal(); err == nil {
			iph.Payload = data
		} else {
			return err
		}

		if data, err := iph.Marshal(); err == nil {
			s.send(data)
		} else {
			return err
		}

		state.ID++

		// non blocking
		select {
		case state.Conn.Connected <- true:
		default:
		}

		state.SocketState = SocketEstablished
	} else if state.SocketState == SocketEstablished {
		if th.Ctrl == tcp.ACK {
		} else {
			iph := ipv4.New().
				WithSource(iph.Dst).
				WithDestination(iph.Src).
				WithID(state.ID)

			th := tcp.Header{
				Source:      th.Destination,
				Destination: th.Source,
				SeqNum:      state.SendNext,
				AckNum:      state.RecvNext,
				DataOffset:  5,
				Reserved:    0,
				ECN:         0,
				Ctrl:        tcp.ACK,
				Window:      64420,
				Checksum:    0,
				Urgent:      0,
				Options:     []tcp.Option{},
				Payload:     []byte{},
			}

			if data, err := th.Marshal(); err == nil {
				iph.Payload = data
			} else {
				return err
			}

			if data, err := iph.Marshal(); err == nil {
				s.send(data)
			} else {
				return err
			}

			state.ID++
		}
		// fmt.Printf("<- Sent ack for: id=%d, seqnum=%d, %d, %d\n", iph.ID, state.SendNext, state.RecvNext, len(iph.Payload))

		if len(th.Payload) > 0 {
			state.Conn.buffer = append(state.Conn.buffer, th.Payload[:]...)

			// non blocking send
			select {
			case state.Conn.Recv <- []byte{}:
			default:
			}
		}

		if th.HasFlag(tcp.FIN) {
			iph := ipv4.Header{
				Version:  4,
				Len:      20,
				TOS:      0,
				TotalLen: 52,
				Flags:    2,
				TTL:      128,
				Protocol: 6,
				Src:      iph.Dst,
				Dst:      iph.Src,
				Options:  []byte{},
				ID:       state.ID,
			}

			th := tcp.Header{
				Source:      th.Destination,
				Destination: th.Source,
				SeqNum:      state.SendNext,
				AckNum:      state.RecvNext,
				DataOffset:  5,
				Reserved:    0,
				ECN:         0,
				Ctrl:        tcp.FIN,
				Window:      64420,
				Checksum:    0,
				Urgent:      0,
				Options:     []tcp.Option{},
				Payload:     []byte{},
			}

			if data, err := th.Marshal(); err == nil {
				iph.Payload = data
			} else {
				return err
			}

			if data, err := iph.Marshal(); err == nil {
				s.send(data)
			} else {
				return err
			}

			state.ID++
			state.SendNext++

			state.SocketState = SocketLastAck // return CloseWait()?

			state.Conn.closing = true
		}
	} else if state.SocketState == SocketFinWait1 {
		if th.HasFlag(tcp.FIN) {
			iph := ipv4.Header{
				Version:  4,
				Len:      20,
				TOS:      0,
				TotalLen: 52,
				Flags:    2,
				TTL:      128,
				Protocol: 6,
				Src:      iph.Dst,
				Dst:      iph.Src,
				Options:  []byte{},
				ID:       state.ID,
			}

			th := tcp.Header{
				Source:      th.Destination,
				Destination: th.Source,
				SeqNum:      state.SendNext,
				AckNum:      state.RecvNext,
				DataOffset:  5,
				Reserved:    0,
				ECN:         0,
				Ctrl:        tcp.ACK,
				Window:      64420,
				Checksum:    0,
				Urgent:      0,
				Options:     []tcp.Option{},
				Payload:     []byte{},
			}

			if data, err := th.Marshal(); err == nil {
				iph.Payload = data
			} else {
				return err
			}

			if data, err := iph.Marshal(); err == nil {
				s.send(data)
			} else {
				return err
			}

			state.ID++

			state.SocketState = SocketClosing
		} else if th.HasFlag(tcp.ACK) {
			state.SocketState = SocketFinWait2
		}
	} else if state.SocketState == SocketFinWait2 {
		iph := ipv4.Header{
			Version:  4,
			Len:      20,
			TOS:      0,
			TotalLen: 52,
			Flags:    2,
			TTL:      128,
			Protocol: 6,
			Src:      iph.Dst,
			Dst:      iph.Src,
			Options:  []byte{},
			ID:       state.ID,
		}

		th := tcp.Header{
			Source:      th.Destination,
			Destination: th.Source,
			SeqNum:      state.SendNext,
			AckNum:      state.RecvNext,
			DataOffset:  5,
			Reserved:    0,
			ECN:         0,
			Ctrl:        tcp.ACK,
			Window:      64420,
			Checksum:    0,
			Urgent:      0,
			Options:     []tcp.Option{},
			Payload:     []byte{},
		}

		if data, err := th.Marshal(); err == nil {
			iph.Payload = data
		} else {
			return err
		}

		if data, err := iph.Marshal(); err == nil {
			s.send(data)
		} else {
			return err
		}

		state.ID++
		state.SocketState = SocketClosing

		state.Conn.close()
	} else if state.SocketState == SocketLastAck {
		if !th.HasFlag(tcp.ACK) {
			return nil
		}

		state.Conn.close()
		state.SocketState = SocketTimeWait
	} else if state.SocketState == SocketClosing {
		if !th.HasFlag(tcp.ACK) {
			return nil
		}

		state.Conn.close()
		state.SocketState = SocketTimeWait
	} else if state.SocketState == SocketTimeWait {
		// timeout
		// then socketstate -> socketclosed
	} else if state.SocketState == SocketClosed {
		fmt.Println("Got packets on closed socket.")
	}

	return nil
}

func (s *Stack) handleUDP(iph *ipv4.Header, data []byte) error {
	return nil
	/*
		hdr := &udp.Header{} // tcp.Header
		hdr.Unmarshal(data)

		// remove checksum, should be recalculated
		hdr.Checksum = 0

		// currently only interested in
		if hdr.Source != 53 && hdr.Destination != 53 {
			fmt.Println("Ignoring port")
			return
		}

		if iph.Src.Equal(net.ParseIP("8.8.8.8")) {
			iph.Src = iph.Dst
			iph.Dst = net.ParseIP("172.16.84.1")
		} else if iph.Src.Equal(net.ParseIP("172.16.84.1")) {
			iph.Src = iph.Dst
			iph.Dst = net.ParseIP("8.8.8.8")
		} else {
			fmt.Println("Unknown traffic", iph.Src.String(), net.ParseIP("172.168.84.1").String())
			return
		}

		// do we have active sockets?
		// send to socket
		// check source address, port with destination address, port

		client := UDPConn{
			closed:      false,
			readBuffer:  hdr.Payload,
			writeBuffer: []byte{},
			iph:         iph,
			hdr:         hdr,
			s:           s,
		}

		server := UDPConn{
			closed:      false,
			readBuffer:  []byte{},
			writeBuffer: []byte{},
			iph:         iph,
			hdr:         hdr,
			s:           s,
		}

		if err := Proxy(&client, &server); err != nil {
			log.Printf("Error proxy: %#v\n", err.Error())
		}
	*/
}
