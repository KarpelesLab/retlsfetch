package retlsfetch_test

import (
	"log"
	"testing"

	"github.com/KarpelesLab/retlsfetch"
)

func TestSaver(t *testing.T) {
	s := retlsfetch.NewSaver()
	err := s.Get("https://ws.atonline.com/.well-known/time")

	if err != nil {
		t.Errorf("failed to get url: %s", err)
	}

	log.Printf("LOG DATA:\n%s", s.Save())
}
