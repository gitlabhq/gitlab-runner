package docutils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type SectionReplacerFN func(in io.Reader) (string, error)

type SectionReplacer struct {
	log *slog.Logger

	input  io.Reader
	output *bytes.Buffer

	sectionBuffer *bytes.Buffer

	startMarker string
	endMarker   string

	startFound bool
	endFound   bool
}

func NewSectionReplacer(name string, in io.Reader) *SectionReplacer {
	return NewSectionReplacerWithLogger(slog.New(slog.NewTextHandler(os.Stderr, nil)), name, in)
}

func NewSectionReplacerWithLogger(log *slog.Logger, name string, in io.Reader) *SectionReplacer {
	startMarker := fmt.Sprintf("<!-- %s_start -->", name)
	endMarker := fmt.Sprintf("<!-- %s_end -->", name)
	log.Debug("Creating new section replacer", "start-marker", startMarker, "end-marker", endMarker)
	return &SectionReplacer{
		log:           log,
		startMarker:   startMarker,
		endMarker:     endMarker,
		input:         in,
		output:        &bytes.Buffer{},
		sectionBuffer: &bytes.Buffer{},
	}
}

func (r *SectionReplacer) Replace(fn SectionReplacerFN) error {
	in := bufio.NewReader(r.input)
	for {
		line, err := in.ReadString('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("reading input stream: %w", err)
		}

		err = r.handleLine(line, fn)
		if err != nil {
			return fmt.Errorf("handling line: %w", err)
		}
	}

	return nil
}

func (r *SectionReplacer) handleLine(line string, fn SectionReplacerFN) error {
	r.log.Debug("Handling line", "line", line)

	r.handleStart(line)
	r.handleRewrite(line)

	return r.handleEnd(line, fn)
}

func (r *SectionReplacer) handleStart(line string) {
	r.log.Debug("executing handleStart")

	if r.startFound || !strings.Contains(line, r.startMarker) {
		return
	}

	r.startFound = true
}

func (r *SectionReplacer) handleRewrite(line string) {
	r.log.Debug("executing handleRewrite")

	if r.startFound && !r.endFound {
		if !strings.Contains(line, r.startMarker) && !strings.Contains(line, r.endMarker) {
			r.sectionBuffer.WriteString(line)
		}
		return
	}

	r.output.WriteString(line)
}

func (r *SectionReplacer) handleEnd(line string, fn SectionReplacerFN) error {
	r.log.Debug("executing handleEnd")

	if !strings.Contains(line, r.endMarker) {
		return nil
	}

	r.endFound = true

	r.log.Debug("Running SectionReplacerFN")
	rewritten, err := fn(r.sectionBuffer)
	if err != nil {
		return fmt.Errorf("calling rewriting function: %w", err)
	}

	r.output.WriteString(r.startMarker + "\n")
	r.output.WriteString(rewritten)
	r.output.WriteString(r.endMarker + "\n")

	return nil
}

func (r *SectionReplacer) Output() string {
	return r.output.String()
}
