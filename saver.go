package retlsfetch

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Saver struct {
	//data   []*saverBuffer
	lk     sync.Mutex
	t      time.Time
	rnd    io.Reader
	dialer net.Dialer
	w      io.Writer
}

func NewSaver(w io.Writer) *Saver {
	now := time.Now()

	s := &Saver{
		t:   now,
		rnd: rand.Reader,
		w:   w,
		//rnd: sha3.NewCShake256(nil, nil), // this random will always return the same bytes
	}

	nowBin, _ := now.MarshalBinary()
	s.append("time", nowBin)
	return s
}

func (s *Saver) httpClient() *http.Client {
	res := &http.Client{
		Transport: &http.Transport{
			DialContext:       s.dialContext,
			DialTLSContext:    s.dialTlsContext,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return res
}

func (s *Saver) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	log.Printf("[saver] dial context: %s %s", network, addr)
	return s.dialer.DialContext(ctx, network, addr)
}

func (s *Saver) dialTlsContext(ctx context.Context, network, addr string) (net.Conn, error) {
	log.Printf("[saver] dial tls context: %s %s", network, addr)
	c, err := s.dialer.DialContext(ctx, network, addr)
	if err != nil {
		s.append(addr+":conn", []byte(err.Error()))
		return nil, err
	}
	s.append(addr+":conn", nil)
	s.appendAddr(addr+":local", c.LocalAddr())
	s.appendAddr(addr+":remote", c.RemoteAddr())
	c = &logConn{
		Conn:   c,
		logger: s,
		name:   addr,
	}
	host, _, _ := net.SplitHostPort(addr)
	cfg := &tls.Config{
		Time:         s.time,
		Rand:         readerFunc(s.ReadRand),
		MinVersion:   tls.VersionTLS12,
		ServerName:   host,
		KeyLogWriter: writerFunc(s.keylog),
	}
	// TODO handshake
	cs := tls.Client(c, cfg)
	err = cs.Handshake()
	if err != nil {
		log.Printf("[saver] handshake failed: %s", err)
		c.Close()
		return nil, err
	}
	return cs, nil
}

func (s *Saver) time() time.Time {
	now := time.Now()
	nowBin, _ := now.MarshalBinary()
	s.append("time", nowBin)
	return now
}

func (s *Saver) append(t string, b []byte) error {
	s.lk.Lock()
	defer s.lk.Unlock()
	log.Printf("[saver] appending %d bytes of %s", len(b), t)

	vint := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(vint, uint64(len(t)))
	_, err := s.w.Write(vint[:n])
	if err != nil {
		return err
	}
	_, err = s.w.Write([]byte(t))
	if err != nil {
		return err
	}

	n = binary.PutUvarint(vint, uint64(len(b)))
	_, err = s.w.Write(vint[:n])
	if err != nil {
		return err
	}
	_, err = s.w.Write(b)
	return err
}

func (s *Saver) appendAddr(t string, addr net.Addr) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		ap := a.AddrPort()
		bin, _ := ap.MarshalBinary()
		s.append(t, bin)
	default:
		panic(fmt.Sprintf("unsupported addr type %T", addr))
	}
}

func (s *Saver) keylog(b []byte) (int, error) {
	s.append("tls:keylog", b)
	return len(b), nil
}

func (s *Saver) Get(u string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	return s.Do(req)
}

func (s *Saver) Do(req *http.Request) (*http.Response, error) {
	resp, err := s.httpClient().Do(req)
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

func (s *Saver) ReadRand(b []byte) (int, error) {
	if len(b) == 1 {
		// workaround for MaybeReadByte in crypto/internal/randutil/randutil.go
		return len(b), nil
	}
	n, err := s.rnd.Read(b)
	if err != nil {
		log.Printf("[rand] error: %s", err)
		return 0, err
	}
	s.append("rnd:read", b[:n])
	return n, err
}
