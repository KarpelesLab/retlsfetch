package retlsfetch_test

import (
	"bytes"
	"log"
	"testing"

	"github.com/KarpelesLab/retlsfetch"
)

func TestSaver(t *testing.T) {
	buf := &bytes.Buffer{}

	s := retlsfetch.NewSaver(buf)
	res1, err := s.Get("https://ws.atonline.com/.well-known/time")

	if err != nil {
		t.Errorf("failed to get url: %s", err)
		return
	}

	log.Printf("tls version: %d", res1.TLS.Version) // , res1.TLS.PeerCertificates[0].Subject)

	//log.Printf("LOG DATA:\n%s", s.Save())

	l := retlsfetch.NewLoader(bytes.NewReader(buf.Bytes()))

	res2, err := l.Get("https://ws.atonline.com/.well-known/time")

	if err != nil {
		t.Errorf("failed to re-get url: %s", err)
		return
	}
	log.Printf("tls version: %d", res2.TLS.Version)
}
