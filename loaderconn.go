package retlsfetch

import (
	"net"
	"time"
)

type loaderConn struct {
	loader *Loader
	name   string
	local  net.Addr
	remote net.Addr
}

func (l *loaderConn) Read(b []byte) (int, error) {
	data, err := l.loader.fetch(l.name + ":read")
	if err != nil {
		return 0, err
	}
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
	return l.local
}

func (l *loaderConn) RemoteAddr() net.Addr {
	return l.remote
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
