package retlsfetch

type readerFunc func(b []byte) (int, error)

func (r readerFunc) Read(b []byte) (int, error) {
	return r(b)
}

type writerFunc func(b []byte) (int, error)

func (w writerFunc) Write(b []byte) (int, error) {
	return w(b)
}

func dup(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	r := make([]byte, len(b))
	copy(r, b)
	return r
}
