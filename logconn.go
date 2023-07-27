package retlsfetch

import "net"

type logConnLogger interface {
	append(t string, b []byte)
}

type logConn struct {
	net.Conn
	logger logConnLogger
	name   string
}

func (l *logConn) Read(b []byte) (int, error) {
	n, err := l.Conn.Read(b)
	if n > 0 {
		l.logger.append(l.name+":read", b[:n])
	}
	return n, err
}

func (l *logConn) Write(b []byte) (int, error) {
	l.logger.append(l.name+":write", b)
	return l.Conn.Write(b)
}
