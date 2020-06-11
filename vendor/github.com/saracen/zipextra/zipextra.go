package zipextra

import (
	"errors"
	"io"
)

// ExtraField is a ZIP Extra Field byte slice.
type ExtraField []byte

// ErrInvalidExtraFieldFormat is returned when the extra field's format is
// incorrect.
var ErrInvalidExtraFieldFormat = errors.New("invalid extra field format")

// Parse parses all extra fields and returns a map with extra field identifiers
// as keys.
func Parse(extra []byte) (map[uint16]ExtraField, error) {
	b := NewBuffer(extra)

	efs := make(map[uint16]ExtraField)
	for b.Available() >= 4 {
		id := uint16(b.Read16())
		size := int(b.Read16())

		if b.Available() < size {
			return efs, io.ErrUnexpectedEOF
		}

		efs[id] = ExtraField(b.Bytes()[:size])
		b.Skip(size)
	}

	return efs, nil
}
