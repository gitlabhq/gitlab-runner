package helpers

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/ajwalker/phrasestream/addmask"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/internal/store"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)
)

type ProxyExecCommand struct {
	Bootstrap bool   `long:"bootstrap" description:"bootstrap helper binary"`
	TempDir   string `long:"temp-dir" description:"temporary directory"`
}

type Proxy struct {
	store   *store.Store
	addmask *addmask.AddMask
}

func NewProxy(dir string, stdout, stderr io.Writer) (*Proxy, error) {
	db, err := store.Open(dir)
	if err != nil {
		return nil, err
	}

	pe := &Proxy{store: db}

	pe.addmask, err = addmask.New(db, stdout, stderr)
	if err != nil {
		return nil, err
	}

	return pe, nil
}

func (p *Proxy) Stdout() io.Writer {
	return p.addmask.Get(0)
}

func (p *Proxy) Stderr() io.Writer {
	return p.addmask.Get(1)
}

func (p *Proxy) Close() error {
	p.store.Close()
	return p.addmask.Close()
}

func (c *ProxyExecCommand) Execute(cliContext *cli.Context) {
	args := cliContext.Args()
	if len(args) == 0 {
		logrus.Fatal("gitlab-runner-helper exec expected args")
	}

	dst := os.Getenv("RUNNER_TEMP_PROJECT_DIR")
	if dst == "" {
		dst = c.TempDir
	}
	if c.Bootstrap {
		if err := bootstrap(dst); err != nil {
			logrus.Fatalln("bootstrapping", err)
		}
	}

	proxy, err := NewProxy(dst, stdout, stderr)
	if err != nil {
		logrus.Fatalln("creating exec proxy", err)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = proxy.Stdout()
	cmd.Stderr = proxy.Stderr()

	err = errors.Join(
		cmd.Run(),
		proxy.Close(),
	)
	if err != nil {
		logrus.Error(err)

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
	}
}

func bootstrap(dst string) error {
	src, _ := os.Executable()

	_ = os.MkdirAll(dst, 0o777)

	pathname := filepath.Join(dst, "gitlab-runner-helper")
	_, err := os.Stat(pathname)
	if err == nil {
		// if the path exists, check to see if it's identical by comparing build info
		buildInfoDst, err := buildinfo.ReadFile(pathname)
		if err != nil {
			return fmt.Errorf("reading build info of existing binary: %w", err)
		}

		buildInfoSrc, ok := debug.ReadBuildInfo()
		if ok && buildInfoDst.String() == buildInfoSrc.String() {
			return nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking helper install: %w", err)
	}

	fsrc, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening helper: %w", err)
	}
	defer fsrc.Close()

	fdst, err := os.CreateTemp(dst, "")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.RemoveAll(fdst.Name())
	defer fdst.Close()

	if _, err := io.Copy(fdst, fsrc); err != nil {
		return fmt.Errorf("copying helper: %w", err)
	}

	if err := fdst.Close(); err != nil {
		return fmt.Errorf("closing helper: %w", err)
	}

	if err := os.Rename(fdst.Name(), pathname); err != nil {
		return fmt.Errorf("renaming helper: %w", err)
	}

	if err := os.Chmod(pathname, 0o777); err != nil {
		return fmt.Errorf("changing helper permissions: %w", err)
	}

	return nil
}

func init() {
	common.RegisterCommand2(
		"proxy-exec",
		"execute internal commands (internal)",
		&ProxyExecCommand{},
	)
}
