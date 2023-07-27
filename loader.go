package retlsfetch

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

type Loader struct {
	lk   sync.Mutex
	data []*saverBuffer
	t    time.Time
}

func (l *Loader) httpClient() *http.Client {
	res := &http.Client{
		Transport: &http.Transport{
			DialContext:       l.dialContext,
			DialTLSContext:    l.dialTlsContext,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return res
}

func (l *Loader) Get(u string) (*http.Response, error) {
	resp, err := l.httpClient().Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	return resp, nil
}

func (l *Loader) fetch(t string) []byte {
	l.lk.Lock()
	defer l.lk.Unlock()

	log.Printf("[loader] fetch %s", t)

	n := 0

	for {
		if len(l.data) <= n {
			panic(fmt.Sprintf("out of data while looking for %s", t))
		}
		c := l.data[n]

		if n == 0 && c.t == "time" {
			l.data = l.data[1:]
			// set l.t
			l.t.UnmarshalBinary(c.b)
			if t == "time" {
				return nil
			}
			continue
		}
		if n == 0 && c.t == "tls:keylog" {
			l.data = l.data[1:]
			continue
		}
		if c.t == t {
			// remove from data
			l.data = append(l.data[:n], l.data[n+1:]...)
			return c.b
		}
		n += 1
	}
}

func (l *Loader) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	log.Printf("[loader] dial context: %s %s", network, addr)
	return nil, errors.New("not tls")
}

func (l *Loader) dialTlsContext(ctx context.Context, network, addr string) (net.Conn, error) {
	log.Printf("[loader] dial tls context: %s %s", network, addr)

	res := l.fetch(addr + ":conn")
	if len(res) > 0 {
		return nil, errors.New(string(res))
	}

	c := &loaderConn{
		loader: l,
		name:   addr,
		local:  spawnAddr(l.fetch(addr + ":local")),
		remote: spawnAddr(l.fetch(addr + ":remote")),
	}
	host, _, _ := net.SplitHostPort(addr)
	cfg := &tls.Config{
		Time:       l.time,
		Rand:       readerFunc(l.readRand),
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	}
	cs := tls.Client(c, cfg)
	err := cs.Handshake()
	if err != nil {
		c.Close()
		return nil, err
	}
	return cs, nil
}

func spawnAddr(b []byte) net.Addr {
	ap := netip.AddrPort{}
	ap.UnmarshalBinary(b)
	return net.TCPAddrFromAddrPort(ap)
}

func (l *Loader) time() time.Time {
	l.fetch("time")
	return l.t
}

func (l *Loader) readRand(b []byte) (int, error) {
	if len(b) == 1 {
		// workaround for MaybeReadByte in crypto/internal/randutil/randutil.go
		return len(b), nil
	}
	data := l.fetch("rnd:read")
	copy(b, data)
	return len(data), nil
}
