package netstack

// +build amd64,linux

import (
	"net"
	_ "net/http/pprof"
)

func (st *StateTable) Add(state *State) {
	*st = append(*st, state)
}

// GetState will return the state for the ip, port combination
func (st *StateTable) Get(SrcIP, DestIP net.IP, SrcPort, DestPort uint16) *State {
	for _, state := range *st {
		if state.SrcPort != SrcPort && state.DestPort != SrcPort {
			continue
		}

		if state.DestPort != DestPort && state.SrcPort != DestPort {
			continue
		}

		// comparing ipv6 with ipv4 now
		if !state.SrcIP.Equal(SrcIP) && !state.DestIP.Equal(SrcIP) {
			continue
		}

		if !state.DestIP.Equal(DestIP) && !state.SrcIP.Equal(DestIP) {
			continue
		}

		return state
	}
	/*

		state := &State{
			SrcIP:  SrcIP,
			DestIP: DestIP,

			SrcPort:  SrcPort,
			DestPort: DestPort,

			SocketState: SocketClosed,
		}

		st.Add(state)
		return state
	*/
	return nil
}

type StateTable []*State

var stateTable StateTable
