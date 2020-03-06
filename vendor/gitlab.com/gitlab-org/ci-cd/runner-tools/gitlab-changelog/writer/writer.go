package writer

import (
	"io"
)

type Writer interface {
	io.Writer

	Flush() error
}
