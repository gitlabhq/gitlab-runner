package zipextra

import (
	"math/big"
)

// ExtraFieldUnixN is the Info-ZIP's New Unix (3rd generation) Extra Field
// identifier.
const ExtraFieldUnixN uint16 = 0x7875

// InfoZIPNewUnix is the New Unix Extra Field structure for holding UID and GID
// file ownership data.
type InfoZIPNewUnix struct {
	Version uint8
	Uid     *big.Int
	Gid     *big.Int
}

// NewInfoZIPNewUnix returns a new InfoZIPNewUnix extra field structure.
func NewInfoZIPNewUnix(uid *big.Int, gid *big.Int) InfoZIPNewUnix {
	return InfoZIPNewUnix{
		Version: 1,
		Uid:     uid,
		Gid:     gid,
	}
}

// Encode encodes the InfoZIPNewUnix extra field.
func (field InfoZIPNewUnix) Encode() []byte {
	buf := NewBuffer([]byte{})
	defer buf.WriteHeader(ExtraFieldUnixN)()

	buid := bigBytesToLittleEndian(field.Uid)
	bgid := bigBytesToLittleEndian(field.Gid)

	buf.WriteByte(field.Version)

	buf.Write8(uint8(len(buid)))
	buf.WriteBytes(buid)

	buf.Write8(uint8(len(bgid)))
	buf.WriteBytes(bgid)

	return buf.Bytes()
}

// InfoZIPNewUnix returns the decoded InfoZIPNewUnix extra field.
func (ef ExtraField) InfoZIPNewUnix() (field InfoZIPNewUnix, err error) {
	buf := NewBuffer(ef)
	if buf.Available() < 3 {
		return field, ErrInvalidExtraFieldFormat
	}

	field.Version = buf.Read8()

	uidSize := int(buf.Read8())
	if buf.Available() < uidSize {
		return field, ErrInvalidExtraFieldFormat
	}
	field.Uid = littleEndianBytesToBig(buf.ReadBytes(uidSize))

	gidSize := int(buf.Read8())
	if buf.Available() < gidSize {
		return field, ErrInvalidExtraFieldFormat
	}
	field.Gid = littleEndianBytesToBig(buf.ReadBytes(gidSize))

	return
}
