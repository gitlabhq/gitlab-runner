package helpers

import (
	"fmt"
	"time"
)

type Printer interface {
	Println(args ...interface{})
}

type BuildSection struct {
	Name string
	Run  func() error
}

const (
	traceSectionStart = "section_start:%v:%s\r" + ANSI_CLEAR
	traceSectionEnd   = "section_end:%v:%s\r" + ANSI_CLEAR
)

func nowUnixUTC() int64 {
	return time.Now().UTC().Unix()
}

func (s *BuildSection) timestamp(format string, logger Printer) {
	sectionLine := fmt.Sprintf(format, nowUnixUTC(), s.Name)
	logger.Println(sectionLine)
}

func (s *BuildSection) start(logger Printer) {
	s.timestamp(traceSectionStart, logger)
}

func (s *BuildSection) end(logger Printer) {
	s.timestamp(traceSectionEnd, logger)
}

func (s *BuildSection) RunAndCollectMetrics(logger Printer) error {
	s.start(logger)
	defer s.end(logger)

	return s.Run()
}
