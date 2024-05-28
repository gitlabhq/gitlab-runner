package docker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

const (
	defaultBuilderName = "buildx-builder"
	defaultContextName = "docker-buildx"
)

var (
	Host     = env.NewDefault("DOCKER_HOST", "unix:///var/run/docker.sock")
	CertPath = env.New("DOCKER_CERT_PATH")

	BuilderEnvBundle = env.Variables{
		Host,
		CertPath,
	}
)

type Builder struct {
	host     string
	certPath string

	builderName string
	contextName string

	retryCount int
}

func NewBuilder(host, certPath string) *Builder {
	return &Builder{
		host:     host,
		certPath: certPath,

		builderName: defaultBuilderName,
		contextName: defaultContextName,
	}
}

func (b *Builder) Docker(args ...string) error {
	return sh.RunWithV(
		map[string]string{
			"DOCKER_CLI_EXPERIMENTAL": "true",
		},
		"docker",
		args...,
	)
}

func (b *Builder) Buildx(args ...string) error {
	return b.Docker(append([]string{"buildx"}, args...)...)
}

func (b *Builder) CleanupContext() error {
	// In the old script this output was suppressed but let's see if there's reason to do so
	// might contain valuable info
	var errs []error
	if err := b.Buildx("rm", b.builderName); err != nil {
		errs = append(errs, err)
	}
	if err := b.Docker("context", "rm", "-f", b.contextName); err != nil {
		errs = append(errs, err)
	}

	return errors.New(strings.Join(lo.Map(errs, func(err error, _ int) string {
		return err.Error()
	}), " "))
}

func (b *Builder) SetupContext() error {
	// We need the context to not exist either way. If we don't clean it up, we just need to rerun the script
	// since it gets deleted in case of an error anyways. There are also some other edge cases where it's not being cleaned up
	// properly so this makes the building of images more consistent and less error prone
	if err := b.CleanupContext(); err != nil {
		fmt.Println("Error cleaning up context:", err)
	}

	// In order for `docker buildx create` to work, we need to replace DOCKER_HOST with a Docker context.
	// Otherwise, we get the following error:
	// > could not create a builder instance with TLS data loaded from environment.

	docker := fmt.Sprintf("host=%s", b.host)
	if b.certPath != "" {
		docker = fmt.Sprintf(
			"host=%s,ca=%s/ca.pem,cert=%s/cert.pem,key=%s/key.pem",
			b.host,
			b.certPath,
			b.certPath,
			b.certPath,
		)
	}

	if err := b.Docker(
		"context", "create", b.contextName,
		"--default-stack-orchestrator", "swarm",
		"--description", "Temporary buildx Docker context",
		"--docker", docker,
	); err != nil {
		return err
	}

	return b.Buildx("create", "--use", "--name", b.builderName, b.contextName)
}

func (b *Builder) Login(username, password, registry string) (func(), error) {
	if username == "" || password == "" {
		return func() {}, nil
	}

	loginCmd := fmt.Sprintf("echo %s | docker login --username %s --password-stdin %s", password, username, registry)
	err := sh.RunV("sh", "-c", loginCmd)
	if err != nil {
		return nil, err
	}

	return func() {
		_ = b.Logout(registry)
	}, nil
}

func (b *Builder) Logout(registry string) error {
	return b.Docker("logout", registry)
}

func (b *Builder) Import(archive, tag, platform, entrypoint string) error {
	fmt.Println("Importing tag", archive, "as", tag, "platform", platform)
	args := []string{"import", archive, tag, "--platform", platform}
	if entrypoint != "" {
		args = append(args, "--change", fmt.Sprintf("ENTRYPOINT %s", entrypoint))
	}

	return b.Docker(args...)
}

func (b *Builder) Tag(tagFrom, tagTo string) error {
	fmt.Println("Tagging image", tagFrom, "as", tagTo)
	return b.Docker("tag", tagFrom, tagTo)
}

func (b *Builder) Push(tag string) error {
	fmt.Println("Pushing image", tag)
	return b.Docker("push", tag)
}
