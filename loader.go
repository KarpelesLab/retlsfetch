package retlsfetch

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
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

type dataRecord struct {
	t string
	b []byte
}

type Loader struct {
	r    *bufio.Reader
	lk   sync.Mutex
	t    time.Time
	data []*dataRecord
}

func NewLoader(r io.Reader) *Loader {
	res := &Loader{
		r: bufio.NewReader(r),
	}
	return res
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
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	return l.Do(req)
}

func (l *Loader) Do(req *http.Request) (*http.Response, error) {
	resp, err := l.httpClient().Do(req)
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

func (l *Loader) loadRecord() (*dataRecord, error) {
	ln, err := binary.ReadUvarint(l.r)
	if err != nil {
		return nil, fmt.Errorf("while reading len: %w", err)
	}
	t := make([]byte, ln)
	_, err = io.ReadFull(l.r, t)
	if err != nil {
		return nil, err
	}

	ln, err = binary.ReadUvarint(l.r)
	if err != nil {
		return nil, err
	}
	b := make([]byte, ln)
	_, err = io.ReadFull(l.r, b)
	if err != nil {
		return nil, err
	}

	return &dataRecord{string(t), b}, nil
}

func (l *Loader) fetch(t string) ([]byte, error) {
	l.lk.Lock()
	defer l.lk.Unlock()

	log.Printf("[loader] fetch %s", t)

	for len(l.data) < 10 {
		c, err := l.loadRecord()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		l.data = append(l.data, c)
	}

	n := 0

	for {
		if len(l.data) <= n {
			return nil, fmt.Errorf("out of data while looking for %s", t)
		}
		c := l.data[n]

		if n == 0 && c.t == "time" {
			l.data = l.data[1:]
			// set l.t
			l.t.UnmarshalBinary(c.b)
			if t == "time" {
				return nil, nil
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
			return c.b, nil
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

	res, err := l.fetch(addr + ":conn")
	if err != nil {
		return nil, err
	}
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
	err = cs.Handshake()
	if err != nil {
		c.Close()
		return nil, err
	}
	return cs, nil
}

func spawnAddr(b []byte, err error) net.Addr {
	if err != nil {
		return nil
	}
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
	data, err := l.fetch("rnd:read")
	if err != nil {
		return 0, err
	}
	copy(b, data)
	return len(data), nil
}
