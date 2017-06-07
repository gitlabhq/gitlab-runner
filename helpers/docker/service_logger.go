package docker_helpers

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

type ServiceLogger struct {
	prefix string
	output io.Writer
}

func (s *ServiceLogger) write(line string) {
	fmt.Fprintf(s.output, "%s[%s]%s %s", helpers.ANSI_BOLD_CYAN, s.prefix, helpers.ANSI_RESET, line)
}

func (s *ServiceLogger) Writer() *io.PipeWriter {
	pipeOut, pipeIn := io.Pipe()
	buffer := bufio.NewReader(pipeOut)

	go func() {
		defer pipeIn.Close()

		for {
			line, err := buffer.ReadString('\n')
			if err == nil || err == io.EOF {
				s.write(line)
				if err == io.EOF {
					return
				}
			} else {
				if !strings.Contains(err.Error(), "bad file descriptor") {
					s.write(fmt.Sprintf("Problem while reading command output: %s\n", err))
				}
				return
			}
		}
	}()

	return pipeIn
}

func NewServiceLogger(prefix string, output io.Writer) *ServiceLogger {
	return &ServiceLogger{
		prefix: prefix,
		output: output,
	}
}
