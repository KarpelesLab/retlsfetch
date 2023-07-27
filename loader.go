package retlsfetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

func (l *Loader) Get(u string) error {
	resp, err := l.httpClient().Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b := make([]byte, 65536)
	for {
		_, err := resp.Body.Read(b)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
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

	c := &loaderConn{
		loader: l,
		name:   addr,
	}
	host, _, _ := net.SplitHostPort(addr)
	cfg := &tls.Config{
		Time:       func() time.Time { log.Printf("time?"); return l.t },
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

func (l *Loader) readRand(b []byte) (int, error) {
	if len(b) == 1 {
		// workaround for MaybeReadByte in crypto/internal/randutil/randutil.go
		return len(b), nil
	}
	data := l.fetch("rnd:read")
	copy(b, data)
	return len(data), nil
}
