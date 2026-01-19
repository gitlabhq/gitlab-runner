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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/stdcopy"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/services"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

type serviceInfo struct {
	ID    string
	Name  string
	IP    []string
	Ports []int
}

type tooManyServicesRequestedError struct {
	requested int
	allowed   int
}

func (e *tooManyServicesRequestedError) Error() string {
	return fmt.Sprintf("too many services requested: %d, only %d allowed", e.requested, e.allowed)
}

func (e *tooManyServicesRequestedError) Is(err error) bool {
	var target *tooManyServicesRequestedError
	if !errors.As(err, &target) {
		return false
	}

	return e.allowed == target.allowed && e.requested == target.requested
}

func (e *executor) createServices() error {
	e.SetCurrentStage(ExecutorStageCreatingServices)
	e.BuildLogger.Debugln("Creating services...")

	servicesDefinitions, err := e.getServicesDefinitions()
	if err != nil {
		return err
	}

	linksMap := make(map[string]*serviceInfo)

	for index, serviceDefinition := range servicesDefinitions {
		if err := e.createFromServiceDefinition(index, serviceDefinition, linksMap); err != nil {
			return err
		}
	}

	e.captureContainersLogs(e.Context, linksMap)

	e.waitForServices()

	for linkName, linkee := range linksMap {
		for _, ip := range linkee.IP {
			e.links = append(e.links, linkName+":"+ip)
		}
	}

	return nil
}

func (e *executor) getServicesDefinitions() (spec.Services, error) {
	var internalServiceImages []string
	serviceDefinitions := spec.Services{}

	for _, service := range e.Config.Docker.GetExpandedServices(e.Build.GetAllVariables()) {
		internalServiceImages = append(internalServiceImages, service.Name)
		serviceDefinitions = append(serviceDefinitions, service.ToImageDefinition())
	}

	for _, service := range e.Build.Services {
		err := e.verifyAllowedImage(service.Name, "services", e.Config.Docker.AllowedServices, internalServiceImages)
		if err != nil {
			return nil, err
		}

		serviceDefinitions = append(serviceDefinitions, service)
	}

	servicesLimit := e.Config.Docker.GetServicesLimit()
	if servicesLimit >= 0 && len(serviceDefinitions) > servicesLimit {
		return nil, &tooManyServicesRequestedError{requested: len(serviceDefinitions), allowed: servicesLimit}
	}

	return serviceDefinitions, nil
}

func (e *executor) waitForServices() {
	timeout := e.Config.Docker.WaitForServicesTimeout
	if timeout == 0 {
		timeout = common.DefaultWaitForServicesTimeout
	}

	// wait for all services to come up
	if timeout > 0 && len(e.services) > 0 {
		e.BuildLogger.Println("Waiting for services to be up and running (timeout", timeout, "seconds)...")
		wg := sync.WaitGroup{}
		for _, service := range e.services {
			wg.Add(1)
			go func(service *serviceInfo) {
				e.waitForServiceContainer(service, time.Duration(timeout)*time.Second)
				wg.Done()
			}(service)
		}
		wg.Wait()
	}
}

func (e *executor) createFromServiceDefinition(
	serviceIndex int,
	serviceDefinition spec.Image,
	linksMap map[string]*serviceInfo,
) error {
	var container *serviceInfo

	serviceMeta := services.SplitNameAndVersion(serviceDefinition.Name)
	if len(serviceDefinition.Aliases()) != 0 {
		serviceMeta.Aliases = append(serviceMeta.Aliases, serviceDefinition.Aliases()...)
	}

	for _, linkName := range serviceMeta.Aliases {
		if linksMap[linkName] != nil {
			e.BuildLogger.Warningln("Service", serviceDefinition.Name, "is already created. Ignoring.")
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

			e.BuildLogger.Debugln("Created service", serviceDefinition.Name, "as", container.ID)
			e.services = append(e.services, container)
			e.temporary = append(e.temporary, container.ID)

			// add 12-character container ID as hostname
			linksMap[container.ID[:min(12, len(container.ID))]] = container
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

func (e *executor) runServiceHealthCheckContainer(service *serviceInfo, timeout time.Duration) error {
	waitImage, err := e.getHelperImage()
	if err != nil {
		return fmt.Errorf("getPrebuiltImage: %w", err)
	}

	containerName := service.Name + "-wait-for-service"

	environment, err := e.addServiceHealthCheckEnvironment(service)
	if err != nil {
		return err
	}

	cmd := []string{"gitlab-runner-helper", "health-check"}

	config := e.createConfigForServiceHealthCheckContainer(service, cmd, waitImage, environment)
	hostConfig := e.createHostConfigForServiceHealthCheck(service)

	e.BuildLogger.Debugln(fmt.Sprintf("Creating service healthcheck container %s...", containerName))
	resp, err := e.dockerConn.ContainerCreate(e.Context, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("create service container: %w", err)
	}
	defer func() { _ = e.removeContainer(e.Context, resp.ID) }()

	e.BuildLogger.Debugln(fmt.Sprintf("Starting service healthcheck container %s (%s)...", containerName, resp.ID))
	err = e.dockerConn.ContainerStart(e.Context, resp.ID, container.StartOptions{})
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
	service *serviceInfo,
	cmd []string,
	waitImage *image.InspectResponse,
	environment []string,
) *container.Config {
	return &container.Config{
		Cmd:    cmd,
		Image:  waitImage.ID,
		Labels: e.labeler.Labels(map[string]string{"type": labelWaitType, "wait": service.ID}),
		Env:    environment,
	}
}

func (e *executor) waitForServiceContainer(service *serviceInfo, timeout time.Duration) {
	start := time.Now()

	err := e.runServiceHealthCheckContainer(service, timeout)
	if err == nil {
		return
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	buffer.WriteString(
		helpers.ANSI_YELLOW + "*** WARNING:" + helpers.ANSI_RESET + " Service " + service.Name +
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

	// The service health checker will keep checking ports for up to the timeout
	// specified above, this gives the container chance to output some logs.
	// However, in the scenario where there is no ports, or some other problem,
	// we need to give the container a little time to emit something of use.
	time.Sleep(min(timeout-time.Since(start), 10*time.Second))

	buffer.WriteString("\n")
	buffer.WriteString("Service container logs:\n")
	buffer.WriteString(e.readContainerLogs(service.ID))
	buffer.WriteString("\n")

	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*********" + helpers.ANSI_RESET + "\n")
	buffer.WriteString("\n")

	wc := e.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stderr)
	defer wc.Close()

	_, _ = wc.Write(buffer.Bytes())
}

// captureContainersLogs initiates capturing logs for the specified containers
// to a desired additional sink. The sink can be any io.Writer. Currently the
// sink is the jobs main trace, which is wrapped in an inlineServiceLogWriter
// instance to add additional context to logs. In the future this could be
// separate file.
func (e *executor) captureContainersLogs(ctx context.Context, linksMap map[string]*serviceInfo) {
	if !e.Build.IsCIDebugServiceEnabled() {
		return
	}

	for _, service := range e.services {
		aliases := []string{}

		for alias, container := range linksMap {
			if alias == container.ID[:min(12, len(container.ID))] {
				// skip if the alias is the container ID:
				// we're only interested in aliases the user provided,
				// not the container ID docker provides.
				continue
			}
			if container == service {
				aliases = append(aliases, alias)
			}
		}

		logger := e.BuildLogger.Stream(buildlogger.StreamStartingServiceLevel, buildlogger.Stdout)
		defer logger.Close()

		sink := service_helpers.NewInlineServiceLogWriter(strings.Join(aliases, "-"), logger)
		if err := e.captureContainerLogs(ctx, service.ID, service.Name, sink); err != nil {
			e.BuildLogger.Warningln(err.Error())
		}
		logger.Close()
	}
}

// captureContainerLogs tails (i.e. reads) logs emitted to stdout or stdin from
// processes in the specified container, and redirects them to the specified
// sink, which can be any io.Writer (e.g. this process's stdout, a file, a log
// aggregator). The logs are streamed as they are emitted, rather than batched
// and written when we disconnect from the container (or it is stopped). The
// specified sink is closed when the source is completely drained.
func (e *executor) captureContainerLogs(ctx context.Context, cid, containerName string, sink io.WriteCloser) error {
	source, err := e.dockerConn.ContainerLogs(ctx, cid, container.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Timestamps: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to open log stream for container %s: %w", containerName, err)
	}

	e.BuildLogger.Debugln("streaming logs for container " + containerName)
	go func() {
		defer source.Close()
		defer sink.Close()

		// Using stdcopy assumes service containers are run with TTY=false. If
		// containers are started with TTY=true, io.Copy should be used instead.
		if _, err := stdcopy.StdCopy(sink, sink, source); err != nil {
			if err != io.EOF && !errors.Is(err, context.Canceled) {
				e.BuildLogger.Warningln(fmt.Sprintf(
					"error streaming logs for container %s: %s",
					containerName,
					err.Error(),
				))
			}
		}
		e.BuildLogger.Debugln("stopped streaming logs for container " + containerName)
	}()
	return nil
}
