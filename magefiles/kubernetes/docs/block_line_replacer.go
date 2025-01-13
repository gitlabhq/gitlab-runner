package docs

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

type blockLineReplacer struct {
	startLine      string
	endLine        string
	replaceContent string

	input  *bytes.Buffer
	output *bytes.Buffer

	startFound bool
	endFound   bool
}

func (r *blockLineReplacer) Replace() (string, error) {
	for {
		line, err := r.input.ReadString('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("error while reading issue description: %w", err)
		}

		r.handleLine(line)
	}

	return r.output.String(), nil
}

func (r *blockLineReplacer) handleLine(line string) {
	r.handleStart(line)
	r.handleRewrite(line)
	r.handleEnd(line)
}

func (r *blockLineReplacer) handleStart(line string) {
	if r.startFound || !strings.Contains(line, r.startLine) {
		return
	}

	r.startFound = true
}

func (r *blockLineReplacer) handleRewrite(line string) {
	if r.startFound && !r.endFound {
		return
	}

	r.output.WriteString(line)
}

func (r *blockLineReplacer) handleEnd(line string) {
	if !strings.Contains(line, r.endLine) {
		return
	}

	r.endFound = true
	r.output.WriteString(r.replaceContent)
}

func NewBlockLineReplacer(startLine, endLine string, input, replaceContent string) *blockLineReplacer {
	return &blockLineReplacer{
		startLine:      startLine,
		endLine:        endLine,
		input:          bytes.NewBufferString(input),
		output:         new(bytes.Buffer),
		replaceContent: replaceContent,
		startFound:     false,
		endFound:       false,
	}
}
