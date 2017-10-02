package helpers

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"strconv"
)

func newSectionParser() *sectionParser {
	parser := &sectionParser{}
	parser.SectionRegexp = regexp.MustCompile("section_((?:start)|(?:end)):(\\d+):(.+)\r\033\\[0K")
	parser.Sections = make([]*parsedSection, 0)

	return parser
}

type parsedSection struct {
	Name  string
	Start time.Time
	End   time.Time
}

func (s *parsedSection) Duration() time.Duration {
	return s.End.Sub(s.Start)
}

type sectionParser struct {
	Sections      []*parsedSection
	SectionRegexp *regexp.Regexp
	Error         error

	currentSection *parsedSection
}

func (p *sectionParser) SendRawLog(args ...interface{}) {
	if p.Error != nil {
		return
	}

	line := fmt.Sprintln(args...)
	matches := p.SectionRegexp.FindStringSubmatch(line)
	if matches != nil {
		folding := matches[1]
		epoch, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			p.Error = err
			return
		}
		section := matches[3]
		timestamp := time.Unix(epoch, 0)

		switch folding {
		case "start":
			if p.currentSection != nil {
				p.Error = fmt.Errorf("Double opening: %s and %s", p.currentSection.Name, section)
				return
			}
			p.currentSection = &parsedSection{Name: section, Start: timestamp}
		case "end":
			if p.currentSection == nil {
				p.Error = fmt.Errorf("End without open: %s", section)
				return
			}

			if p.currentSection.Name != section {
				p.Error = fmt.Errorf("Section name mismatch: start %s - end %s", p.currentSection.Name, section)
				return
			}

			p.currentSection.End = timestamp
			p.Sections = append(p.Sections, p.currentSection)
			p.currentSection = nil
		}
	}
}

func TestBuildSection(t *testing.T) {
	for num, tc := range []struct {
		name  string
		delay time.Duration
		error error
	}{
		{"MyTest", time.Second, nil},
		{"MyFailingTest", time.Second, fmt.Errorf("Failing test")},
		{"0_time", 0 * time.Second, nil},
	} {
		parser := newSectionParser()

		section := BuildSection{Name: tc.name, Run: func() error { time.Sleep(tc.delay); return tc.error }}
		section.Execute(parser)

		assert.Nil(t, parser.Error, "case %d: Error: %s", num, parser.Error)
		assert.Equal(t, 1, len(parser.Sections), "case %d: wrong number of sections detected", num)
		firstSection := parser.Sections[0]
		assert.Equal(t, tc.name, firstSection.Name, "case %d: wrong name", num)
		assert.Equal(t, tc.delay, firstSection.Duration(), "case %d: wrong duration")
	}
}

func TestBuildSectionSkipMetrics(t *testing.T) {
	parser := newSectionParser()

	section := BuildSection{Name: "SkipMetrics", SkipMetrics: true, Run: func() error { return nil }}
	section.Execute(parser)

	assert.Nil(t, parser.Error)
	assert.Equal(t, 0, len(parser.Sections))
}
