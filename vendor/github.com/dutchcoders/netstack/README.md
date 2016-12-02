# netstack
Custom network stack in Go

This networkstack implements (very) basic and rudimentary support for connecting tcp streams. There needs to be a lot to be implemented and optimised.

# Configuration (for now)

Linux will send RST packets for unknown tcp packets, so this needs to be disabled using iptables. 

```
iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP
iptables -I OUTPUT -p icmp --icmp-type destination-unreachable -j DROP
```

# Samples

See samples folder.

# References

* http://backreference.org/2010/03/26/tuntap-interface-tutorial/
* http://stackoverflow.com/questions/3062205/setting-the-source-ip-for-a-udp-socket
* http://stackoverflow.com/questions/12177708/raw-socket-promiscuous-mode-not-sniffing-what-i-write
* http://stackoverflow.com/questions/110341/tcp-handshake-with-sock-raw-socket
* http://devdungeon.com/content/packet-capture-injection-and-analysis-gopacket
* https://en.wikipedia.org/wiki/Transmission_Control_Protocol#/media/File:Tcp_state_diagram_fixed_new.svg
* https://github.com/mindreframer/golang-stuff/blob/master/github.com/pebbe/zmq2/examples/udpping1.go
* https://github.com/adamdunkels/uip/blob/master/uip/uip.c
* http://www.darkcoding.net/uncategorized/raw-sockets-in-go-ip-layer/
* http://lxr.free-electrons.com/source/net/ipv4/tcp.c<F37>
* https://www.freebsd.org/doc/en/books/developers-handbook/sockets-essential-functions.html
* http://stackoverflow.com/questions/8047728/how-to-set-linux-kernel-not-to-send-rst-ack-so-that-i-can-give-syn-ack-within-r


