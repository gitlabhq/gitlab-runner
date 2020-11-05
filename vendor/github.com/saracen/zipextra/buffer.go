package zipextra

import (
	"math/big"
	"time"
)

// Buffer provides primitive read/write functions for extra fields.
type Buffer struct {
	buf   []byte
	index int
}

// NewBuffer returns a new Buffer.
func NewBuffer(e []byte) *Buffer {
	return &Buffer{buf: e}
}

// Available returns the number of unread bytes available.
func (p *Buffer) Available() int {
	return len(p.buf[p.index:])
}

// Bytes returns the unread bytes.
func (p *Buffer) Bytes() []byte {
	return p.buf[p.index:]
}

// Skip skips n bytes.
func (p *Buffer) Skip(n int) {
	p.index += n
}

// ReadBytes reads the specified number of bytes.
func (p *Buffer) ReadBytes(size int) (b []byte) {
	b = p.buf[p.index : p.index+size]
	p.index += size
	return b
}

// Read8 reads and returns a uint8.
func (p *Buffer) Read8() (n uint8) {
	n = p.buf[p.index]
	p.index++
	return n
}

// Read16 reads and returns a uint16.
func (p *Buffer) Read16() (n uint16) {
	n |= uint16(p.buf[p.index])
	n |= uint16(p.buf[p.index+1]) << 8
	p.index += 2
	return n
}

// Read32 reads and returns a uint32.
func (p *Buffer) Read32() (n uint32) {
	n |= uint32(p.buf[p.index])
	n |= uint32(p.buf[p.index+1]) << 8
	n |= uint32(p.buf[p.index+2]) << 16
	n |= uint32(p.buf[p.index+3]) << 24
	p.index += 4
	return n
}

// WriteByte writes a single byte.
func (p *Buffer) WriteByte(b byte) {
	p.buf = append(p.buf, b)
}

// WriteBytes writes multiple bytes.
func (p *Buffer) WriteBytes(b []byte) {
	p.buf = append(p.buf, b...)
}

// Write8 writes a uint8.
func (p *Buffer) Write8(n uint8) {
	p.buf = append(p.buf, n)
}

// Write8 writes a uint16.
func (p *Buffer) Write16(n uint16) {
	p.buf = append(p.buf, uint8(n), uint8(n>>8))
}

// Write8 writes a uint32.
func (p *Buffer) Write32(n uint32) {
	p.buf = append(p.buf, uint8(n), uint8(n>>8), uint8(n>>16), uint8(n>>24))
}

// WriteHeader writes a standard Zip Extra Field header, consisting of a uint16
// identifier, and uint16 size.
func (p *Buffer) WriteHeader(id uint16) func() {
	p.Write16(id)
	p.Write16(0)
	return func() {
		size := len(p.Bytes()) - 4
		p.buf[2] = uint8(size)
		p.buf[3] = uint8(size >> 8)
	}
}

func bigBytesToLittleEndian(x *big.Int) []byte {
	b := x.Bytes()
	for i := len(b)/2 - 1; i >= 0; i-- {
		opp := len(b) - 1 - i
		b[i], b[opp] = b[opp], b[i]
	}
	return b
}

func littleEndianBytesToBig(b []byte) *big.Int {
	for i := len(b)/2 - 1; i >= 0; i-- {
		opp := len(b) - 1 - i
		b[i], b[opp] = b[opp], b[i]
	}
	return big.NewInt(0).SetBytes(b)
}

func timeToFiletime(t time.Time) (uint32, uint32) {
	nsec := t.UnixNano()
	nsec /= 100
	nsec += 116444736000000000

	return uint32(nsec & 0xffffffff), uint32(nsec >> 32 & 0xffffffff)
}

func filetimeToTime(l uint32, h uint32) time.Time {
	nsec := int64(h)<<32 + int64(l)
	nsec -= 116444736000000000
	nsec *= 100

	return time.Unix(0, nsec)
}
