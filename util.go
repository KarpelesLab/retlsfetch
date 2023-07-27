package retlsfetch

type readerFunc func(b []byte) (int, error)

func (r readerFunc) Read(b []byte) (int, error) {
	return r(b)
}

func dup(b []byte) []byte {
	r := make([]byte, len(b))
	copy(r, b)
	return r
}
