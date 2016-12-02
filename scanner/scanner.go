// +build amd64,linux

package scanner

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	_ "log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"sync"
	"time"

	"github.com/bogdanovich/dns_resolver"
	"github.com/fatih/color"

	"github.com/dutchcoders/anam/config"
	"github.com/dutchcoders/netstack"
)

type Host struct {
	Name string
	IP   net.IP
}

type Scanner struct {
	hostsCh         chan string
	resolvedHostsCh chan Host

	resolver *dns_resolver.DnsResolver
	s        *netstack.Stack
	config   *config.Config
}

func New(config *config.Config) (*Scanner, error) {
	a := Scanner{
		hostsCh:         make(chan string, 100),
		resolvedHostsCh: make(chan Host, 100),

		config: config,
	}

	if v, err := netstack.New(config.Interface); err != nil {
		return nil, err
	} else {
		a.s = v
	}

	if resolver, err := dns_resolver.NewFromResolvConf("/etc/resolv.conf"); err != nil {
		return nil, err
	} else {
		resolver.RetryTimes = 5

		a.resolver = resolver
	}

	return &a, nil
}

func (a *Scanner) SetResolver(resolver *dns_resolver.DnsResolver) {
	a.resolver = resolver
}

func (a *Scanner) resolve(ctx context.Context) {
	q := make(chan struct{}, 100)
	defer close(a.resolvedHostsCh)

	var wg sync.WaitGroup

	for host := range a.hostsCh {
		q <- struct{}{}
		wg.Add(1)

		go func(h string) {
			defer func() {
				wg.Done()
				<-q
			}()

			a.lookup(h)
		}(host)
	}

	wg.Wait()
}

func (a *Scanner) lookup(h string) {
	prefixes := strings.Split(a.config.Prefixes, ",")
	prefixes = append([]string{""}, prefixes...)

	for _, prefix := range prefixes {
		host := h
		if prefix != "" {
			host = strings.Join([]string{prefix, h}, ".")
		}

		if ips, err := a.resolver.LookupHost(host); err != nil {
			color.Red("Could not resolve host (%s): %s", host, err.Error())
		} else if len(ips) == 0 {
		} else {
			for _, dest := range ips {
				a.resolvedHostsCh <- Host{
					Name: host,
					IP:   dest,
				}
			}
		}
	}
}

func (a *Scanner) connect(h Host) (net.Conn, error) {
	if conn, err := a.s.Connect(h.IP, a.config.Port); err != nil {
		return nil, err
	} else if !a.config.UseTLS {
		return conn, nil
	} else {
		tlsconn := tls.Client(conn, &tls.Config{
			ServerName:         h.Name,
			InsecureSkipVerify: true,
		})

		if err := tlsconn.Handshake(); err != nil {
			return nil, err
		}

		return tlsconn, nil
	}
}

func (a *Scanner) scan(host Host) {
	conn, err := a.connect(host)
	if err != nil {
		color.Red("[%s]: Connect failed (%s): %s", host.Name, host.IP.String(), err.Error())
		return
	}

	defer conn.Close()

	for _, path := range a.config.Paths {
		payload := []byte(fmt.Sprintf("GET %s HTTP/1.1\r\nUser-Agent: %s\r\nHost: %s\r\nAccept: */*\r\n\r\n", path, a.config.UserAgent, host.Name))
		if _, err := conn.Write([]byte(payload)); err != nil {
			color.Red("Connection write %s: %s", host, err.Error())
			break
		}

		r := io.TeeReader(conn, ioutil.Discard)
		if resp, err := http.ReadResponse(bufio.NewReader(r), nil); err != nil {
			color.Red("Read response %s: %s", host, err.Error())
			return
		} else if data, err := ioutil.ReadAll(resp.Body); err != nil {
			// ignore error
			color.Red("ReadAll %s: %s", host, err.Error())
			return
		} else {
			str := string(data[0:20])
			color.Yellow("Got statuscode %d for host %s(%s) on path %s: %s.", resp.StatusCode, host.Name, host.IP.String(), path, str)
		}
	}
}

func (a *Scanner) Scan(ctx context.Context) {
	// start the network stack
	a.s.Start()
	defer a.s.Close()

	go a.resolve(ctx)

	start := time.Now()

	// thread limiter
	ch := make(chan struct{}, a.config.NumThreads)

	// do we need a feedback ch?
	count := 0

	var wg sync.WaitGroup

	go func() {
		<-ctx.Done()
		if ctx.Err() == context.Canceled {
			close(a.hostsCh)

			color.Yellow("Waiting for scans to finish.")
			wg.Wait()
		}
	}()

	// this should be rewritten to not use goroutines, but just return a channel with input for connection
	for host := range a.resolvedHostsCh {
		if count == 0 {
		} else if count%100 == 0 {
			ms := int(time.Now().Sub(start) / time.Millisecond)
			color.Yellow("Checked %d hosts in %vs, avg=%vms per domain.\n", count, ms/1000, ms/count)
		}

		ch <- struct{}{}
		wg.Add(1)

		go func(host Host) {
			defer func() {
				<-ch
				wg.Done()
			}()

			a.scan(host)
		}(host)

		count++
	}

	wg.Wait()
}

func (a *Scanner) Feed() chan string {
	return a.hostsCh
}
