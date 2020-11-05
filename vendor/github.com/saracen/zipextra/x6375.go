package zipextra

import "hash/crc32"

// ExtraFieldUCom is the Info-ZIP's Unicode Comment Extra Field identifier.
const ExtraFieldUCom uint16 = 0x6375

// InfoZIPUnicodeComment is the Unicode Comment Extra Field structure for
// holding NTFS file comments.
type InfoZIPUnicodeComment struct {
	Version uint8
	Crc32   uint32
	Comment string
}

// NewInfoZIPUnicodeComment returns a new InfoZIPUnicodeComment extra field
// structure.
func NewInfoZIPUnicodeComment(comment string) InfoZIPUnicodeComment {
	return InfoZIPUnicodeComment{
		Version: 1,
		Crc32:   crc32.ChecksumIEEE([]byte(comment)),
		Comment: comment,
	}
}

// Encode encodes the InfoZIPUnicodeComment extra field.
func (field InfoZIPUnicodeComment) Encode() []byte {
	buf := NewBuffer([]byte{})
	defer buf.WriteHeader(ExtraFieldUCom)()

	buf.Write8(field.Version)
	buf.Write32(field.Crc32)
	buf.WriteBytes([]byte(field.Comment))

	return buf.Bytes()
}

// InfoZIPUnicodeComment returns the decoded InfoZIPUnicodeComment extra field.
func (ef ExtraField) InfoZIPUnicodeComment() (field InfoZIPUnicodeComment, err error) {
	buf := NewBuffer(ef)
	if buf.Available() < 5 {
		return field, ErrInvalidExtraFieldFormat
	}

	field.Version = buf.Read8()
	field.Crc32 = buf.Read32()
	field.Comment = string(buf.ReadBytes(buf.Available()))

	return
}
