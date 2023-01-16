package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/services"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

func (e *executor) createServices() error {
	e.SetCurrentStage(ExecutorStageCreatingServices)
	e.Debugln("Creating services...")

	servicesDefinitions, err := e.getServicesDefinitions()
	if err != nil {
		return err
	}

	linksMap := make(map[string]*types.Container)

	for index, serviceDefinition := range servicesDefinitions {
		if err := e.createFromServiceDefinition(index, serviceDefinition, linksMap); err != nil {
			return err
		}
	}

	e.captureContainersLogs(e.Context, linksMap)

	e.waitForServices()

	if e.networkMode.IsBridge() || e.networkMode.NetworkName() == "" {
		e.Debugln("Building service links...")
		e.links = e.buildServiceLinks(linksMap)
	}

	return nil
}

func (e *executor) getServicesDefinitions() (common.Services, error) {
	var internalServiceImages []string
	serviceDefinitions := common.Services{}

	for _, service := range e.Config.Docker.Services {
		internalServiceImages = append(internalServiceImages, service.Name)
		serviceDefinitions = append(serviceDefinitions, service.ToImageDefinition())
	}

	for _, service := range e.Build.Services {
		serviceName := e.Build.GetAllVariables().ExpandValue(service.Name)
		err := e.verifyAllowedImage(serviceName, "services", e.Config.Docker.AllowedServices, internalServiceImages)
		if err != nil {
			return nil, err
		}

		service.Name = serviceName
		serviceDefinitions = append(serviceDefinitions, service)
	}

	return serviceDefinitions, nil
}

func (e *executor) waitForServices() {
	waitForServicesTimeout := e.Config.Docker.WaitForServicesTimeout
	if waitForServicesTimeout == 0 {
		waitForServicesTimeout = common.DefaultWaitForServicesTimeout
	}

	// wait for all services to came up
	if waitForServicesTimeout > 0 && len(e.services) > 0 {
		e.Println("Waiting for services to be up and running (timeout", waitForServicesTimeout, "seconds)...")
		wg := sync.WaitGroup{}
		for _, service := range e.services {
			wg.Add(1)
			go func(service *types.Container) {
				_ = e.waitForServiceContainer(service, time.Duration(waitForServicesTimeout)*time.Second)
				wg.Done()
			}(service)
		}
		wg.Wait()
	}
}

func (e *executor) buildServiceLinks(linksMap map[string]*types.Container) (links []string) {
	for linkName, linkee := range linksMap {
		newContainer, err := e.client.ContainerInspect(e.Context, linkee.ID)
		if err != nil {
			continue
		}
		if newContainer.State.Running {
			links = append(links, linkee.ID+":"+linkName)
		}
	}
	return
}

func (e *executor) createFromServiceDefinition(
	serviceIndex int,
	serviceDefinition common.Image,
	linksMap map[string]*types.Container,
) error {
	var container *types.Container

	serviceMeta := services.SplitNameAndVersion(serviceDefinition.Name)
	if len(serviceDefinition.Aliases()) != 0 {
		serviceMeta.Aliases = append(serviceMeta.Aliases, serviceDefinition.Aliases()...)
	}

	for _, linkName := range serviceMeta.Aliases {
		if linksMap[linkName] != nil {
			e.Warningln("Service", serviceDefinition.Name, "is already created. Ignoring.")
			continue
		}

		// Create service if not yet created
		if container == nil {
			var err error
			container, err = e.createService(
				serviceIndex,
				serviceMeta.Service,
				serviceMeta.Version,
				serviceMeta.ImageName,
				serviceDefinition,
				serviceMeta.Aliases,
			)
			if err != nil {
				return err
			}

			e.Debugln("Created service", serviceDefinition.Name, "as", container.ID)
			e.services = append(e.services, container)
			e.temporary = append(e.temporary, container.ID)
		}
		linksMap[linkName] = container
	}
	return nil
}

type serviceHealthCheckError struct {
	Inner error
	Logs  string
}

func (e *serviceHealthCheckError) Error() string {
	if e.Inner == nil {
		return "serviceHealthCheckError"
	}

	return e.Inner.Error()
}

func (e *executor) runServiceHealthCheckContainer(service *types.Container, timeout time.Duration) error {
	waitImage, err := e.getPrebuiltImage()
	if err != nil {
		return fmt.Errorf("getPrebuiltImage: %w", err)
	}

	containerName := service.Names[0] + "-wait-for-service"

	environment, err := e.addServiceHealthCheckEnvironment(service)
	if err != nil {
		return err
	}

	cmd := []string{"gitlab-runner-helper", "health-check"}

	config := e.createConfigForServiceHealthCheckContainer(service, cmd, waitImage, environment)
	hostConfig := e.createHostConfigForServiceHealthCheck(service)

	e.Debugln(fmt.Sprintf("Creating service healthcheck container %s...", containerName))
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("create service container: %w", err)
	}
	defer func() { _ = e.removeContainer(e.Context, resp.ID) }()

	e.Debugln(fmt.Sprintf("Starting service healthcheck container %s (%s)...", containerName, resp.ID))
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("start service container: %w", err)
	}

	ctx, cancel := context.WithTimeout(e.Context, timeout)
	defer cancel()

	err = e.waiter.Wait(ctx, resp.ID)
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("service %q timeout", containerName)
	} else {
		err = fmt.Errorf("service %q health check: %w", containerName, err)
	}

	return &serviceHealthCheckError{
		Inner: err,
		Logs:  e.readContainerLogs(resp.ID),
	}
}

func (e *executor) createConfigForServiceHealthCheckContainer(
	service *types.Container,
	cmd []string,
	waitImage *types.ImageInspect,
	environment []string,
) *container.Config {
	return &container.Config{
		Cmd:    cmd,
		Image:  waitImage.ID,
		Labels: e.labeler.Labels(map[string]string{"type": labelWaitType, "wait": service.ID}),
		Env:    environment,
	}
}

func (e *executor) waitForServiceContainer(service *types.Container, timeout time.Duration) error {
	err := e.runServiceHealthCheckContainer(service, timeout)
	if err == nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	buffer.WriteString(
		helpers.ANSI_YELLOW + "*** WARNING:" + helpers.ANSI_RESET + " Service " + service.Names[0] +
			" probably didn't start properly.\n")
	buffer.WriteString("\n")
	buffer.WriteString("Health check error:\n")
	buffer.WriteString(strings.TrimSpace(err.Error()))
	buffer.WriteString("\n")

	if healtCheckErr, ok := err.(*serviceHealthCheckError); ok {
		buffer.WriteString("\n")
		buffer.WriteString("Health check container logs:\n")
		buffer.WriteString(healtCheckErr.Logs)
		buffer.WriteString("\n")
	}

	buffer.WriteString("\n")
	buffer.WriteString("Service container logs:\n")
	buffer.WriteString(e.readContainerLogs(service.ID))
	buffer.WriteString("\n")

	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*********" + helpers.ANSI_RESET + "\n")
	buffer.WriteString("\n")
	_, _ = io.Copy(e.Trace, &buffer)
	return err
}

// captureContainersLogs initiates capturing logs for the specified containers
// to a desired additional sink. The sink can be any io.Writer. Currently the
// sink is the jobs main trace, which is wrapped in an inlineServiceLogWriter
// instance to add additional context to logs. In the future this could be
// separate file.
func (e *executor) captureContainersLogs(ctx context.Context, linksMap map[string]*types.Container) {
	if !e.Build.IsCIDebugServiceEnabled() {
		return
	}

	for _, service := range e.services {
		aliases := []string{}

		for alias, container := range linksMap {
			if container == service {
				aliases = append(aliases, alias)
			}
		}

		sink := service_helpers.NewInlineServiceLogWriter(strings.Join(aliases, "-"), e.Trace)
		if err := e.captureContainerLogs(ctx, service.ID, service.Names[0], sink); err != nil {
			e.Warningln(err.Error())
		}
	}
}

// captureContainerLogs tails (i.e. reads) logs emitted to stdout or stdin from
// processes in the specified container, and redirects them to the specified
// sink, which can be any io.Writer (e.g. this process's stdout, a file, a log
// aggregator). The logs are streamed as they are emitted, rather than batched
// and written when we disconnect from the container (or it is stopped). The
// specified sink is closed when the source is completely drained.
func (e *executor) captureContainerLogs(ctx context.Context, cid, containerName string, sink io.WriteCloser) error {
	source, err := e.client.ContainerLogs(ctx, cid, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to open log stream for container %s: %w", containerName, err)
	}

	e.Debugln("streaming logs for container " + containerName)
	go func() {
		defer source.Close()
		defer sink.Close()

		// Using stdcopy assumes service containers are run with TTY=false. If
		// containers are started with TTY=true, io.Copy should be used instead.
		if _, err := stdcopy.StdCopy(sink, sink, source); err != nil {
			if err != io.EOF && !errors.Is(err, context.Canceled) {
				e.Warningln(fmt.Sprintf("error streaming logs for container %s: %s", containerName, err.Error()))
			}
		}
		e.Debugln("stopped streaming logs for container " + containerName)
	}()
	return nil
}
