package retlsfetch

import (
	"net"
	"time"
)

type loaderConn struct {
	loader *Loader
	name   string
}

func (l *loaderConn) Read(b []byte) (int, error) {
	data := l.loader.fetch(l.name + ":read")
	copy(b, data)
	return len(data), nil
}

func (l *loaderConn) Write(b []byte) (int, error) {
	l.loader.fetch(l.name + ":write")
	// TODO compare
	return len(b), nil
}

func (l *loaderConn) Close() error {
	return nil
}

func (l *loaderConn) LocalAddr() net.Addr {
	// TODO
	return nil
}

func (l *loaderConn) RemoteAddr() net.Addr {
	// TODO
	return nil
}

func (l *loaderConn) SetDeadline(time.Time) error {
	return nil
}

func (l *loaderConn) SetReadDeadline(time.Time) error {
	return nil
}

func (l *loaderConn) SetWriteDeadline(time.Time) error {
	return nil
}
