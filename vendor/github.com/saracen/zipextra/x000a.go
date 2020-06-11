package zipextra

import (
	"time"
)

// NTFS Extra Field identifier
const ExtraFieldNTFS uint16 = 0x000a

// NTFS is the NTFS Extra Field structure for holding NTFS attributes.
type NTFS struct {
	Attributes []NTFSAttribute
}

// NTFSAttribute is an NTFS attribute interface. Each attribute consists of a
// tag and data. The tag is an identifier for the type of data.
type NTFSAttribute interface {
	Tag() uint16
	Data() []byte
}

// NTFSRawAttribute represents an unrecognized NTFS attribute.
type NTFSRawAttribute struct {
	RawTag  uint16
	RawData []byte
}

// Tag returns the unrecognized NTFS attribute's tag.
func (a NTFSRawAttribute) Tag() uint16 {
	return a.RawTag
}

// Tag returns the unrecognized NTFS attribute's data.
func (a NTFSRawAttribute) Data() []byte {
	return a.RawData
}

// NTFSTimeAttribute is an NTFS attribute for storing modified, access and
// creation file times.
type NTFSTimeAttribute struct {
	MTime time.Time
	ATime time.Time
	CTime time.Time
}

// Tag returns the NTFS Time attribute's tag. It is always 1.
func (a NTFSTimeAttribute) Tag() uint16 {
	return 1
}

// Data returns the NTFS Time attribute's encoded data.
func (a NTFSTimeAttribute) Data() []byte {
	buf := NewBuffer(make([]byte, 0, 8*3))
	for _, t := range []time.Time{a.MTime, a.ATime, a.CTime} {
		l, h := timeToFiletime(t)
		buf.Write32(l)
		buf.Write32(h)
	}

	return buf.Bytes()
}

// NewNTFS returns a new NTFS extra field structure.
func NewNTFS(attributes ...NTFSAttribute) NTFS {
	return NTFS{
		Attributes: attributes,
	}
}

// Encode encodes the NTFS extra field.
func (field NTFS) Encode() []byte {
	buf := NewBuffer([]byte{})
	defer buf.WriteHeader(ExtraFieldNTFS)()

	buf.Write32(0) // reserved

	for _, attribute := range field.Attributes {
		buf.Write16(attribute.Tag())
		buf.Write16(uint16(len(attribute.Data())))
		buf.WriteBytes(attribute.Data())
	}

	return buf.Bytes()
}

// NTFS returns the decoded NTFS extra field.
func (ef ExtraField) NTFS() (field NTFS, err error) {
	buf := NewBuffer(ef)
	if buf.Available() < 4 {
		return field, ErrInvalidExtraFieldFormat
	}

	buf.Read32() // reserved

	for buf.Available() >= 2 {
		tag := buf.Read16()
		size := buf.Read16()

		if buf.Available() < int(size) {
			return field, ErrInvalidExtraFieldFormat
		}

		switch tag {
		case 1:
			if size < 24 {
				return field, ErrInvalidExtraFieldFormat
			}

			field.Attributes = append(field.Attributes, NTFSTimeAttribute{
				MTime: filetimeToTime(buf.Read32(), buf.Read32()),
				ATime: filetimeToTime(buf.Read32(), buf.Read32()),
				CTime: filetimeToTime(buf.Read32(), buf.Read32()),
			})

		default:
			field.Attributes = append(field.Attributes, NTFSRawAttribute{
				RawTag:  tag,
				RawData: buf.ReadBytes(int(size)),
			})
		}
	}
	return
}
