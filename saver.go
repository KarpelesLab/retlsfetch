package retlsfetch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/sha3"
)

type saverBuffer struct {
	t string
	b []byte
}

func (s *saverBuffer) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{s.t, s.b})
}

type Saver struct {
	data   []*saverBuffer
	t      time.Time
	rnd    io.Reader
	dialer net.Dialer
}

func NewSaver() *Saver {
	now := time.Now()

	s := &Saver{
		t:   now,
		rnd: sha3.NewCShake256(nil, nil), // this random will always return the same bytes
	}

	nowBin, _ := now.MarshalBinary()
	s.append("time", nowBin)
	return s
}

func (s *Saver) Loader() *Loader {
	return &Loader{
		data: s.data,
		t:    s.t,
	}
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
		return nil, err
	}
	c = &logConn{
		Conn:   c,
		logger: s,
		name:   addr,
	}
	host, _, _ := net.SplitHostPort(addr)
	cfg := &tls.Config{
		Time:         func() time.Time { log.Printf("time?"); return s.t },
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

func (s *Saver) Save() []byte {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.Encode(s.data)
	return buf.Bytes()
}

func (s *Saver) append(t string, b []byte) {
	log.Printf("[saver] appending %d bytes of %s", len(b), t)
	s.data = append(s.data, &saverBuffer{t, dup(b)})
}

func (s *Saver) keylog(b []byte) (int, error) {
	s.append("tls:keylog", b)
	return len(b), nil
}

func (s *Saver) Get(u string) error {
	resp, err := s.httpClient().Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b := make([]byte, 65536)
	for {
		n, err := resp.Body.Read(b)
		if n > 0 {
			s.append("http:body", b[:n])
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
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
