package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

var images = []string{
	"alpine:3.14.2",
	"registry.gitlab.com/gitlab-org/gitlab-runner/alpine-no-root:latest",
	"docker:23-git",
	"docker:23-dind",
}

type rewriter struct {
	ctx    context.Context
	prefix string
	input  *bufio.Reader
}

func (rw *rewriter) watch() {
	for {
		select {
		case <-rw.ctx.Done():
			return
		case err := <-rw.rewriteInput():
			if err != nil {
				rw.writeToOutput(fmt.Sprintf("Error while reading command output: %v", err))
				return
			}
		}
	}
}

func (rw *rewriter) rewriteInput() <-chan error {
	e := make(chan error)

	go func() {
		line, err := rw.input.ReadString('\n')
		if err == nil || err == io.EOF {
			rw.writeToOutput(line)
			e <- nil

			return
		}

		e <- err
	}()

	return e
}

func (rw *rewriter) writeToOutput(line string) {
	fmt.Printf("\u001B[0;33m[%s]\u001B[0;m %s", rw.prefix, line)
}

func newRewriter(ctx context.Context, prefix string) io.Writer {
	pr, pw, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	w := &rewriter{
		ctx:    ctx,
		prefix: prefix,
		input:  bufio.NewReader(pr),
	}

	go w.watch()

	return pw
}

func pullImage(wg *sync.WaitGroup, name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		wg.Done()
	}()

	output := newRewriter(ctx, name)

	cmd := exec.Command("docker", "pull", name)
	cmd.Stdout = output
	cmd.Stderr = output

	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func main() {
	wg := new(sync.WaitGroup)

	for _, image := range images {
		wg.Add(1)
		go pullImage(wg, image)
	}

	wg.Wait()
}
