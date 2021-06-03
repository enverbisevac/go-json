package json

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/goccy/go-json/internal/decoder"
	"github.com/goccy/go-json/internal/runtime"
)

type Decoder struct {
	s *decoder.Stream
}

const (
	nul = '\000'
)

type emptyInterface struct {
	typ *runtime.Type
	ptr unsafe.Pointer
}

func unmarshal(data []byte, v interface{}) error {
	src := make([]byte, len(data)+1) // append nul byte to the end
	copy(src, data)

	header := (*emptyInterface)(unsafe.Pointer(&v))

	if err := validateType(header.typ, uintptr(header.ptr)); err != nil {
		return err
	}
	dec, err := decoder.CompileToGetDecoder(header.typ)
	if err != nil {
		return err
	}
	cursor, err := dec.Decode(src, 0, 0, header.ptr)
	if err != nil {
		return err
	}
	return validateEndBuf(src, cursor)
}

func unmarshalNoEscape(data []byte, v interface{}) error {
	src := make([]byte, len(data)+1) // append nul byte to the end
	copy(src, data)

	header := (*emptyInterface)(unsafe.Pointer(&v))

	if err := validateType(header.typ, uintptr(header.ptr)); err != nil {
		return err
	}
	dec, err := decoder.CompileToGetDecoder(header.typ)
	if err != nil {
		return err
	}
	cursor, err := dec.Decode(src, 0, 0, noescape(header.ptr))
	if err != nil {
		return err
	}
	return validateEndBuf(src, cursor)
}

func validateEndBuf(src []byte, cursor int64) error {
	for {
		switch src[cursor] {
		case ' ', '\t', '\n', '\r':
			cursor++
			continue
		case nul:
			return nil
		}
		return errSyntax(
			fmt.Sprintf("invalid character '%c' after top-level value", src[cursor]),
			cursor+1,
		)
	}
}

//nolint:staticcheck
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func validateType(typ *runtime.Type, p uintptr) error {
	if typ == nil || typ.Kind() != reflect.Ptr || p == 0 {
		return &InvalidUnmarshalError{Type: runtime.RType2Type(typ)}
	}
	return nil
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the JSON values requested.
func NewDecoder(r io.Reader) *Decoder {
	s := decoder.NewStream(r)
	return &Decoder{
		s: s,
	}
}

// Buffered returns a reader of the data remaining in the Decoder's
// buffer. The reader is valid until the next call to Decode.
func (d *Decoder) Buffered() io.Reader {
	return d.s.Buffered()
}

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for Unmarshal for details about
// the conversion of JSON into a Go value.
func (d *Decoder) Decode(v interface{}) error {
	header := (*emptyInterface)(unsafe.Pointer(&v))
	typ := header.typ
	ptr := uintptr(header.ptr)
	typeptr := uintptr(unsafe.Pointer(typ))
	// noescape trick for header.typ ( reflect.*rtype )
	copiedType := *(**runtime.Type)(unsafe.Pointer(&typeptr))

	if err := validateType(copiedType, ptr); err != nil {
		return err
	}

	dec, err := decoder.CompileToGetDecoder(typ)
	if err != nil {
		return err
	}
	if err := d.s.PrepareForDecode(); err != nil {
		return err
	}
	s := d.s
	if err := dec.DecodeStream(s, 0, header.ptr); err != nil {
		return err
	}
	s.Reset()
	return nil
}

func (d *Decoder) More() bool {
	return d.s.More()
}

func (d *Decoder) Token() (Token, error) {
	return d.s.Token()
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (d *Decoder) DisallowUnknownFields() {
	d.s.DisallowUnknownFields = true
}

func (d *Decoder) InputOffset() int64 {
	return d.s.TotalOffset()
}

// UseNumber causes the Decoder to unmarshal a number into an interface{} as a
// Number instead of as a float64.
func (d *Decoder) UseNumber() {
	d.s.UseNumber = true
}
