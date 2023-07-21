package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/alexflint/go-arg"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

type args struct {
	Revision    string             `arg:"--revision, required" help:"Revision or commit of images to sync e.g. (e.g. v16.0.0 | a54hf6)"`
	Concurrency int                `arg:"--concurrency" help:"The amount of concurrent image pushes to be done" default:"1"`
	Command     spaceSeparatedList `arg:"--command,env:SYNC_COMMAND" help:"The Command that will be executed to sync the images. Can be multiple strings separated by a space. Default (skopeo)"`
	Images      commaSeparatedList `arg:"--images" help:"Comma separated list of which types of images to sync - runner, helper. Default: (runner,helper)"`
	Filters     commaSeparatedList `arg:"--filters" help:"Comma separated list of tag regexp filters to be applied to the images to be synced. Empty by default"`
	IsLatest    bool               `arg:"--is-latest" help:"Also sync -latest images"`
	DryRun      bool               `arg:"-n,--dry-run" help:"Print commands to be run without actually running them"`

	compiledFilters []*regexp.Regexp
}

func (a *args) compileFilters() {
	a.compiledFilters = lo.Map(a.Filters, func(filter string, _ int) *regexp.Regexp {
		return regexp.MustCompile(filter)
	})
}

type image string
type registry string

type registryInfo struct {
	url string
	env string
}

type targetImage struct {
	variants    []string
	destination image
	source      image
}

type tag struct {
	name        string
	destination image
	source      image
}

type imageSyncPair struct {
	from, to string
}

type commaSeparatedList []string

//nolint:unparam
func (c *commaSeparatedList) UnmarshalText(b []byte) error {
	values := lo.Map(strings.Split(string(b), ","), func(val string, _ int) string {
		return strings.TrimSpace(val)
	})
	*c = values

	return nil
}

type spaceSeparatedList []string

//nolint:unparam
func (c *spaceSeparatedList) UnmarshalText(b []byte) error {
	values := lo.Map(strings.Split(string(b), " "), func(val string, _ int) string {
		return strings.TrimSpace(val)
	})
	*c = values

	return nil
}

const (
	gitlabRunnerImage       image = "gitlab-runner"
	gitlabRunnerHelperImage image = "gitlab-runner/gitlab-runner-helper"

	gitlabRunnerDestinationImage       image = "gitlab-runner"
	gitlabRunnerHelperDestinationImage image = "gitlab-runner-helper"

	registryDockerHub registry = "DockerHub"
	registryECR       registry = "ECR"

	envCIRegistryImage = "CI_REGISTRY_IMAGE"

	envPushToECR         = "PUSH_TO_ECR_PUBLIC"
	envECRPublicRegistry = "ECR_PUBLIC_REGISTRY"
	envECRPublicUser     = "ECR_PUBLIC_USER"

	envPushToDockerHub   = "PUSH_TO_DOCKER_HUB"
	envDockerHubRegistry = "DOCKER_HUB_REGISTRY"
	envDockerHubUser     = "DOCKER_HUB_USER"
	envDockerHubPassword = "DOCKER_HUB_PASSWORD"
	envIsLatest          = "IS_LATEST"
)

var (
	sourceRegistry        = envOr(envCIRegistryImage, "registry.gitlab.com/gitlab-org")
	ecrPublicRegistryUser = envOr(envECRPublicUser, "AWS")

	targetRegistries = map[registry]registryInfo{
		registryDockerHub: {
			url: envOr(envDockerHubRegistry, "registry.hub.docker.com/gitlab"),
			env: envPushToDockerHub,
		},
		registryECR: {
			url: envOr(envECRPublicRegistry, "public.ecr.aws/gitlab"),
			env: envPushToECR,
		},
	}

	runnerVariantsTemplatePath = "runner_variants_list.tpl"
	helperVariantsTemplatePath = "runner_helper_variants_list.tpl"
)

func envOr(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}

	return fallback
}

func getVariantsFromTemplate(path string, revision string, isLatest bool) ([]string, error) {
	fullpath := path
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		fullpath = filepath.Join("sync-docker-images", path)
	}

	tpl, err := template.New(path).ParseFiles(fullpath)
	if err != nil {
		return nil, err
	}

	type context struct {
		Revision string
		IsLatest bool
	}

	var out bytes.Buffer

	if err := tpl.Execute(&out, context{Revision: revision, IsLatest: isLatest}); err != nil {
		return nil, err
	}

	return lo.Filter(strings.Split(out.String(), "\n"), func(variant string, _ int) bool {
		// Ignore empty lines and comments
		return variant != "" && !strings.HasPrefix(variant, "#")
	}), nil
}

func generateTargetImages(args *args) (map[image]targetImage, error) {
	runnerVariants, err := getVariantsFromTemplate(runnerVariantsTemplatePath, args.Revision, args.IsLatest)
	if err != nil {
		return nil, err
	}

	helperVariants, err := getVariantsFromTemplate(helperVariantsTemplatePath, args.Revision, args.IsLatest)
	if err != nil {
		return nil, err
	}

	return map[image]targetImage{
		gitlabRunnerImage: {
			variants:    runnerVariants,
			destination: gitlabRunnerDestinationImage,
			source:      gitlabRunnerImage,
		},
		gitlabRunnerHelperImage: {
			variants:    helperVariants,
			destination: gitlabRunnerHelperDestinationImage,
			source:      gitlabRunnerHelperImage,
		},
	}, nil
}

func newImageSyncPair(sourceRegistry, toRegistry string, fromImg, toImg image, tag string) imageSyncPair {
	return imageSyncPair{
		from: fmt.Sprintf("%s/%s:%s", sourceRegistry, fromImg, tag),
		to:   fmt.Sprintf("%s/%s:%s", toRegistry, toImg, tag),
	}
}

func main() {
	args := parseArgs()

	if err := syncImages(args); err != nil {
		log.Fatalln(err)
	}
}

func parseArgs() *args {
	var args args

	arg.MustParse(&args)
	if args.Concurrency <= 0 {
		args.Concurrency = 1
	}

	if len(args.Command) == 0 {
		args.Command = []string{"skopeo"}
	}

	if len(args.Images) == 0 {
		args.Images = []string{"runner", "helper"}
	}

	for i, img := range args.Images {
		switch img {
		case "runner":
			args.Images[i] = string(gitlabRunnerImage)
		case "helper":
			args.Images[i] = string(gitlabRunnerHelperImage)
		}
	}

	args.compileFilters()

	if !args.IsLatest {
		args.IsLatest, _ = strconv.ParseBool(envOr(envIsLatest, "false"))
	}

	log.Printf("Will sync images: %+v\n", args.Images)

	return &args
}

func runCmd(args *args, cmd *exec.Cmd) error {
	if args.DryRun {
		fmt.Printf("Cmd: %s\n", cmd)
		return nil
	}

	return cmd.Run()
}

func outputCmd(args *args, cmd *exec.Cmd) ([]byte, error) {
	if args.DryRun {
		fmt.Printf("Cmd: %s\n", cmd)
		return nil, nil
	}

	return cmd.Output()
}

func generateImageSyncPairs(tags []tag, registries map[registry]string) []imageSyncPair {
	var images []imageSyncPair

	for _, registry := range registries {
		for _, tag := range tags {
			images = append(images, newImageSyncPair(sourceRegistry, registry, tag.source, tag.destination, tag.name))
		}
	}

	return images
}

func generateAllTags(args *args) ([]tag, error) {
	targets, err := generateTargetImages(args)
	if err != nil {
		return nil, err
	}

	var tags []tag

	for img, target := range targets {
		if !lo.Contains(args.Images, string(img)) {
			continue
		}

		for _, variant := range target.variants {
			if !filterVariant(variant, args.compiledFilters) {
				continue
			}

			tags = append(tags, tag{
				name:        variant,
				source:      target.source,
				destination: target.destination,
			})
		}
	}

	return tags, nil
}

func filterVariant(variant string, filters []*regexp.Regexp) bool {
	for _, filter := range filters {
		if !filter.MatchString(variant) {
			return false
		}
	}

	return true
}

func filterTargetRegistries() map[registry]string {
	registries := make(map[registry]string)

	for registry, info := range targetRegistries {
		if enabled, _ := strconv.ParseBool(os.Getenv(info.env)); enabled {
			registries[registry] = info.url
		}
	}

	return registries
}

func syncImages(args *args) error {
	registries := filterTargetRegistries()
	if len(registries) == 0 {
		log.Printf("Warn: No registries to push to, check the values of %q and %q\n", envPushToDockerHub, envPushToECR)
		return nil
	}

	err := loginRegistries(args, registries)
	if err != nil {
		return err
	}

	tags, err := generateAllTags(args)
	if err != nil {
		return err
	}

	images := generateImageSyncPairs(tags, registries)

	pool := pool.New().WithErrors().WithMaxGoroutines(args.Concurrency)

	for _, pair := range images {
		pair := pair // Create local copy to avoid data race with loop variable
		pool.Go(func() error {
			log.Printf("Copying image %s => %s", pair.from, pair.to)

			cmd := buildCmd(
				append(
					args.Command,
					[]string{
						"copy",
						"--all",
						fmt.Sprintf("docker://%s", pair.from),
						fmt.Sprintf("docker://%s", pair.to),
					}...)...,
			)

			return runCmd(args, cmd)
		})
	}

	return pool.Wait()
}

func loginRegistries(args *args, registries map[registry]string) error {
	for registry, addr := range registries {
		log.Printf("Logging into %s:%s", registry, addr)
		switch registry {
		case registryDockerHub:
			if err := loginRegistry(args, addr, os.Getenv(envDockerHubUser), os.Getenv(envDockerHubPassword)); err != nil {
				return err
			}
		case registryECR:
			cmd := buildCmdNoStdout(
				"aws",
				"--region",
				"us-east-1",
				"ecr-public",
				"get-login-password",
			)

			ecrPassword, err := outputCmd(args, cmd)
			if err != nil {
				return fmt.Errorf("getting ecr password for %s: %w", addr, err)
			}

			if err := loginRegistry(args, addr, ecrPublicRegistryUser, string(ecrPassword)); err != nil {
				return err
			}
		default:
		}
	}

	return nil
}

func loginRegistry(args *args, addr string, username, password string) error {
	cmd := buildCmd(
		append(args.Command, []string{
			"login",
			addr,
			"-u",
			username,
			"-p",
			password,
		}...)...,
	)

	return runCmd(args, cmd)
}

func buildCmd(commandAndArgs ...string) *exec.Cmd {
	cmd := exec.Command(commandAndArgs[0], commandAndArgs[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func buildCmdNoStdout(commandAndArgs ...string) *exec.Cmd {
	cmd := exec.Command(commandAndArgs[0], commandAndArgs[1:]...)

	cmd.Stderr = os.Stderr

	return cmd
}
