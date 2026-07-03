package gitproto

import (
	"errors"
	"fmt"
	"io"
)

// LimitPackReader wraps a ReadCloser with a byte limit. Shared across strategies.
func LimitPackReader(r io.ReadCloser, maxBytes int64) io.ReadCloser {
	if maxBytes <= 0 {
		return r
	}
	return &packLimitRC{ReadCloser: r, max: maxBytes}
}

type packLimitRC struct {
	io.ReadCloser

	max  int64
	read int64
}

func (r *packLimitRC) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.read += int64(n)
	if r.read > r.max {
		return n, fmt.Errorf("source pack exceeded max-pack-bytes limit (%d)", r.max)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return n, fmt.Errorf("read: %w", err)
	}
	return n, err //nolint:wrapcheck // io.EOF must pass through for io.Reader contract
}
