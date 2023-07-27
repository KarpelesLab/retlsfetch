package retlsfetch_test

import (
	"bytes"
	"testing"

	"github.com/KarpelesLab/retlsfetch"
)

func TestSaver(t *testing.T) {
	buf := &bytes.Buffer{}

	s := retlsfetch.NewSaver(buf)
	_, err := s.Get("https://ws.atonline.com/.well-known/time")

	if err != nil {
		t.Errorf("failed to get url: %s", err)
	}

	//log.Printf("LOG DATA:\n%s", s.Save())

	l := retlsfetch.NewLoader(bytes.NewReader(buf.Bytes()))

	_, err = l.Get("https://ws.atonline.com/.well-known/time")

	if err != nil {
		t.Errorf("failed to re-get url: %s", err)
	}
}
