package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/exec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/user"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/limitwriter"
)

type commandExecutor struct {
	executor
	buildContainer                  *types.ContainerJSON
	lock                            sync.Mutex
	terminalWaitForContainerTimeout time.Duration
}

func (s *commandExecutor) getBuildContainer() *types.ContainerJSON {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.buildContainer
}

func (s *commandExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.executor.Prepare(options)
	if err != nil {
		return err
	}

	s.Debugln("Starting Docker command...")

	if len(s.BuildShell.DockerCommand) == 0 {
		return errors.New("script is not compatible with Docker")
	}

	_, err = s.getPrebuiltImage()
	if err != nil {
		return err
	}

	_, err = s.getBuildImage()
	if err != nil {
		return err
	}

	if s.isUmaskDisabled() {
		s.Println("Not using umask - FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR is set!")
	}

	return nil
}

func (s *commandExecutor) isUmaskDisabled() bool {
	// Not usable with docker-windows executor
	if s.AbstractExecutor.ExecutorOptions.Metadata[metadataOSType] == osTypeWindows {
		return false
	}

	if !s.Build.IsFeatureFlagOn(featureflags.DisableUmaskForDockerExecutor) {
		return false
	}

	return true
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) error {
	maxAttempts, err := s.Build.GetExecutorJobSectionAttempts()
	if err != nil {
		return fmt.Errorf("getting job section attempts: %w", err)
	}

	var runErr error
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		if attempts > 1 {
			s.Infoln(fmt.Sprintf("Retrying %s", cmd.Stage))
		}

		ctr, err := s.getContainer(cmd)
		if err != nil {
			return err
		}

		s.Debugln("Executing on", ctr.Name, "the", cmd.Script)
		s.SetCurrentStage(ExecutorStageRun)

		runErr = s.startAndWatchContainer(cmd.Context, ctr.ID, bytes.NewBufferString(cmd.Script))
		if !docker.IsErrNotFound(runErr) {
			return runErr
		}

		s.Errorln(fmt.Sprintf("Container %q not found or removed. Will retry...", ctr.ID))
	}

	if runErr != nil && maxAttempts > 1 {
		s.Errorln("Execution attempts exceeded")
	}

	return runErr
}

func (s *commandExecutor) getContainer(cmd common.ExecutorCommand) (*types.ContainerJSON, error) {
	if cmd.Predefined {
		return s.requestNewPredefinedContainer()
	}

	return s.requestBuildContainer()
}

func (s *commandExecutor) requestNewPredefinedContainer() (*types.ContainerJSON, error) {
	prebuildImage, err := s.getPrebuiltImage()
	if err != nil {
		return nil, err
	}

	buildImage := common.Image{
		Name: prebuildImage.ID,
	}

	containerJSON, err := s.createContainer("predefined", buildImage, s.getHelperImageCmd(), []string{prebuildImage.ID})
	if err != nil {
		return nil, err
	}

	return containerJSON, err
}

func (s *commandExecutor) getHelperImageCmd() []string {
	if s.isUmaskDisabled() {
		return []string{"/bin/bash"}
	}

	return s.helperImageInfo.Cmd
}

func (s *commandExecutor) requestBuildContainer() (*types.ContainerJSON, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.buildContainer != nil {
		_, inspectErr := s.client.ContainerInspect(s.Context, s.buildContainer.ID)
		if inspectErr == nil {
			return s.buildContainer, nil
		}

		if !docker.IsErrNotFound(inspectErr) {
			s.Warningln("Failed to inspect build container", s.buildContainer.ID, inspectErr.Error())
		}
	}

	var err error
	s.buildContainer, err = s.createContainer("build", s.Build.Image, s.BuildShell.DockerCommand, []string{})
	if err != nil {
		return nil, err
	}

	err = s.changeFilesOwnership()
	if err != nil {
		return nil, err
	}

	return s.buildContainer, nil
}

func (s *commandExecutor) changeFilesOwnership() error {
	if !s.isUmaskDisabled() {
		return nil
	}

	dockerExec := exec.NewDocker(s.Context, s.client, s.waiter, s.Build.Log())
	inspect := user.NewInspect(s.client, dockerExec)
	imageSHA := s.buildContainer.Image
	imageName := s.Build.Image.Name

	log := s.Build.Log().WithFields(logrus.Fields{
		"imageSHA":  imageSHA,
		"imageName": imageName,
	})
	log.Debug("Checking if image runs with root user")

	usesRoot, err := inspect.IsRoot(s.Context, imageSHA)
	if err != nil {
		return fmt.Errorf("checking if image %q runs as root: %w", imageName, err)
	}

	if usesRoot {
		log.Debug("Image uses root user")
		return nil
	}

	log.Debug("Image doesn't use root user")

	uid, gid, err := getUIDandGID(s.Context, log, inspect, s.buildContainer.ID, imageSHA)
	if err != nil {
		return err
	}

	if uid == 0 {
		return nil
	}

	return s.executeChown(dockerExec, uid, gid)
}

func getUIDandGID(
	ctx context.Context,
	log logrus.FieldLogger,
	inspect user.Inspect,
	buildContainerID string,
	imageSHA string,
) (int, int, error) {
	containerLog := log.WithField("container", buildContainerID)
	containerLog.Debug("Getting the UID of the container")

	uid, err := inspect.UID(ctx, buildContainerID)
	if err != nil {
		return 0, 0, fmt.Errorf("checking %q image UID: %w", imageSHA, err)
	}

	containerLog.Debugf("Container UID=%d", uid)
	containerLog.Debug("Getting the GID of the container")

	gid, err := inspect.GID(ctx, buildContainerID)
	if err != nil {
		return 0, 0, fmt.Errorf("checking %q image GID: %w", imageSHA, err)
	}

	containerLog.Debugf("Container GID=%d", gid)

	return uid, gid, err
}

func (s *commandExecutor) executeChown(dockerExec exec.Docker, uid int, gid int) error {
	c, err := s.requestNewPredefinedContainer()
	if err != nil {
		return fmt.Errorf("requesting new predefined container: %w", err)
	}

	err = s.executeChownOnDir(c, dockerExec, uid, gid, s.Build.FullProjectDir())
	if err != nil {
		return err
	}

	err = s.executeChownOnDir(c, dockerExec, uid, gid, s.Build.TmpProjectDir())
	if err != nil {
		return err
	}

	return nil
}

func (s *commandExecutor) executeChownOnDir(
	c *types.ContainerJSON,
	dockerExec exec.Docker,
	uid int,
	gid int,
	dir string,
) error {
	s.Println(fmt.Sprintf("Changing ownership of files at %q to %d:%d", dir, uid, gid))

	output := new(bytes.Buffer)
	// limit how much data we read from the container log to
	// avoid memory exhaustion
	lw := limitwriter.New(output, 1024)
	streams := exec.IOStreams{
		Stdin:  strings.NewReader(fmt.Sprintf("chown -RP -- %d:%d %q", uid, gid, dir)),
		Stderr: lw,
		Stdout: lw,
	}

	err := dockerExec.Exec(s.Context, c.ID, streams)

	log := s.Build.Log().WithField("updatedDir", dir)
	log.WithField("output", output.String()).Debug("Changing ownership of files")

	if err != nil {
		log.WithError(err).Error("Failed to change ownership of files")
	}

	return nil
}

func (s *commandExecutor) GetMetricsSelector() string {
	return fmt.Sprintf("instance=%q", s.executor.info.Name)
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              "/builds",
		DefaultCacheDir:               "/cache",
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
		Metadata: map[string]string{
			metadataOSType: osTypeLinux,
		},
	}

	creator := func() common.Executor {
		e := &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
				volumeParser: parser.NewLinuxParser(),
			},
		}

		e.newVolumePermissionSetter = func() (permission.Setter, error) {
			helperImage, err := e.getPrebuiltImage()
			if err != nil {
				return nil, err
			}

			return permission.NewDockerLinuxSetter(e.client, e.Build.Log(), helperImage), nil
		}

		e.SetCurrentStage(common.ExecutorStageCreated)
		return e
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Image = true
		features.Services = true
		features.Session = true
		features.Terminal = true
		features.ServiceVariables = true
		features.ServiceMultipleAliases = true
	}

	common.RegisterExecutorProvider("docker", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		ConfigUpdater:    configUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
