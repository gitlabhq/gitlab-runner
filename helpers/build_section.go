package helpers

import (
	"fmt"
	"time"
)

type RawLogger interface {
	SendRawLog(args ...any)
}

type BuildSection struct {
	Name        string
	SkipMetrics bool
	Collapsed   bool
	Run         func() error
}

const (
	traceSectionStart = "section_start:%v:%s\r" + ANSI_CLEAR
	traceSectionEnd   = "section_end:%v:%s\r" + ANSI_CLEAR
)

func nowUnixUTC() int64 {
	return time.Now().UTC().Unix()
}

func (s *BuildSection) timestamp(format, name string, logger RawLogger) {
	if s.SkipMetrics {
		return
	}

	sectionLine := fmt.Sprintf(format, nowUnixUTC(), name)
	logger.SendRawLog(sectionLine)
}

func (s *BuildSection) start(logger RawLogger) {
	name := s.Name
	if s.Collapsed {
		name += "[collapsed=true]"
	}

	s.timestamp(traceSectionStart, name, logger)
}

func (s *BuildSection) end(logger RawLogger) {
	s.timestamp(traceSectionEnd, s.Name, logger)
}

func (s *BuildSection) Execute(logger RawLogger) error {
	s.start(logger)
	defer s.end(logger)

	return s.Run()
}
