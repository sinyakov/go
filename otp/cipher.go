// +build !solution

package otp

import (
	"errors"
	"io"
)

type xorReader struct {
	r    io.Reader
	prng io.Reader
}

func (x *xorReader) Read(p []byte) (n int, err error) {
	n, err = x.r.Read(p)

	bufPrng := make([]byte, n)
	bufPrngLen, prngErr := io.ReadFull(x.prng, bufPrng)

	if prngErr != nil {
		return 0, prngErr
	}

	if bufPrngLen < n {
		return 0, errors.New("random bytes less than buffer length")
	}

	for i := 0; i < n; i++ {
		p[i] ^= bufPrng[i]
	}

	return n, err
}

type xorWriter struct {
	w    io.Writer
	prng io.Reader
}

func (x *xorWriter) Write(p []byte) (n int, err error) {
	length := len(p)
	buf := make([]byte, length)
	copy(buf, p)

	bufPrng := make([]byte, length)
	bufPrngLen, prngErr := io.ReadFull(x.prng, bufPrng)

	if prngErr != nil {
		return 0, prngErr
	}

	if bufPrngLen < length {
		return 0, errors.New("random bytes less than buffer length")
	}

	for i := 0; i < length; i++ {
		buf[i] ^= bufPrng[i]
	}

	return x.w.Write(buf)
}

func NewReader(r io.Reader, prng io.Reader) io.Reader {
	return &xorReader{r: r, prng: prng}
}

func NewWriter(w io.Writer, prng io.Reader) io.Writer {
	return &xorWriter{w: w, prng: prng}
}
