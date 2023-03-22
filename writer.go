package query

import (
	"io"
	"unicode/utf8"
)

type writer struct {
	discard bool

	inner    io.Writer
	buf      []byte
	ptr      int
	wipeable bool
}

func writeTo(w io.Writer) *writer {
	return &writer{
		discard: true,
		inner:   w,
		buf:     make([]byte, 4096),
	}
}

func (w *writer) writeRune(r rune) {
	if w.discard {
		return
	}
	z := utf8.RuneLen(r)
	if w.ptr+z >= len(w.buf) {
		w.flush()
	}
	utf8.EncodeRune(w.buf[w.ptr:], r)
	w.ptr += z
	w.wipeable = true
}

func (w *writer) unwriteRune() {
	if !w.wipeable {
		return
	}
	if w.discard || w.ptr == 0 {
		return
	}
	_, z := utf8.DecodeLastRune(w.buf[:w.ptr])
	w.ptr -= z
	w.wipeable = false
}

func (w *writer) toggle() {
	w.flush()
	w.discard = !w.discard
}

func (w *writer) close() {
	w.flush()
	if w.ptr == 0 {
		return
	}
	w.inner.Write(w.buf[:w.ptr])
}

func (w *writer) flush() {
	if w.ptr <= 0 || w.discard {
		return
	}
	r, z := utf8.DecodeLastRune(w.buf[:w.ptr])
	if r == utf8.RuneError {
		return
	}
	w.inner.Write(w.buf[:w.ptr-z])
	w.ptr = 0
	if r != 0 {
		utf8.EncodeRune(w.buf, r)
		w.ptr = z
	}
}