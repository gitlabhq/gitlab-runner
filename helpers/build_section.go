package helpers

import (
	"fmt"
	"time"
)

type RawLogger interface {
	SendRawLog(args ...interface{})
}

type BuildSection struct {
	Name        string
	SkipMetrics bool
	Run         func() error
}

const (
	traceSectionStart = "section_start:%v:%s\r" + ANSI_CLEAR
	traceSectionEnd   = "section_end:%v:%s\r" + ANSI_CLEAR
)

func nowUnixUTC() int64 {
	return time.Now().UTC().Unix()
}

func (s *BuildSection) timestamp(format string, logger RawLogger) {
	if s.SkipMetrics {
		return
	}

	sectionLine := fmt.Sprintf(format, nowUnixUTC(), s.Name)
	logger.SendRawLog(sectionLine)
}

func (s *BuildSection) start(logger RawLogger) {
	s.timestamp(traceSectionStart, logger)
}

func (s *BuildSection) end(logger RawLogger) {
	s.timestamp(traceSectionEnd, logger)
}

func (s *BuildSection) Execute(logger RawLogger) error {
	s.start(logger)
	defer s.end(logger)

	return s.Run()
}
