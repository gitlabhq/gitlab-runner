package common

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
	traceSectionStart = "\rsection_start:%v:%s\r"
	traceSectionEnd   = "\rsection_end:%v:%s\r"
)

func nowUnixUTC() int64 {
	return time.Now().UTC().Unix()
}

func (s *BuildSection) RunAndCollectMetrics(logger Printer) error {
	sectionLine := fmt.Sprintf(traceSectionStart, nowUnixUTC(), s.Name)
	logger.Println(sectionLine)

	err := s.Run()

	sectionLine = fmt.Sprintf(traceSectionEnd, nowUnixUTC(), s.Name)
	logger.Println(sectionLine)

	return err
}
