package retlsfetch

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
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
	lk     sync.Mutex
	t      time.Time
	rnd    io.Reader
	dialer net.Dialer
}

func NewSaver() *Saver {
	now := time.Now()

	s := &Saver{
		t:   now,
		rnd: rand.Reader,
		//rnd: sha3.NewCShake256(nil, nil), // this random will always return the same bytes
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

func (s *Saver) Save() []byte {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.Encode(s.data)
	return buf.Bytes()
}

func (s *Saver) WriteTo(w io.Writer) (int64, error) {
	var n, n2 int
	var total int64
	var err error
	vint := make([]byte, binary.MaxVarintLen64)

	// write data as binary format
	for _, d := range s.data {
		n = binary.PutUvarint(vint, uint64(len(d.t)))
		n2, err = w.Write(vint[:n])
		total += int64(n2)
		if err != nil {
			return total, err
		}
		n2, err = w.Write([]byte(d.t))
		total += int64(n2)
		if err != nil {
			return total, err
		}
		n = binary.PutUvarint(vint, uint64(len(d.b)))
		n2, err = w.Write(vint[:n])
		total += int64(n2)
		if err != nil {
			return total, err
		}
		n2, err = w.Write(d.b)
		total += int64(n2)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (s *Saver) append(t string, b []byte) {
	s.lk.Lock()
	defer s.lk.Unlock()
	log.Printf("[saver] appending %d bytes of %s", len(b), t)
	s.data = append(s.data, &saverBuffer{t, dup(b)})
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
