package main

import (
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type arch string
type variant string
type image string
type registry string

const (
	gitlabRunnerImage       image = "gitlab-runner"
	gitlabRunnerHelperImage image = "gitlab-runner/gitlab-runner-helper"

	gitlabRunnerDestinationImage       image = "gitlab-runner"
	gitlabRunnerHelperDestinationImage image = "gitlab-runner-helper"

	archARM     arch = "arm"
	archARM64   arch = "arm64"
	archPPC64LE arch = "ppc64le"
	archS390X   arch = "s390x"
	archX8664   arch = "x86_64"

	variantPWSH           variant = "pwsh"
	variantServerCore1809 variant = "servercore1809"
	variantServerCore21H2 variant = "servercore21H2"

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
)

var (
	sourceRegistry        = envOr(envCIRegistryImage, "registry.gitlab.com/gitlab-org")
	ecrPublicRegistryUser = envOr(envECRPublicUser, "AWS")
)

func envOr(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}

	return fallback
}

var targetRegistries = map[registry]string{
	registryDockerHub: envOr(envDockerHubRegistry, "registry.hub.docker.com/gitlab"),
	registryECR:       envOr(envECRPublicRegistry, "public.ecr.aws/gitlab"),
}

var supportedArchitectures = []arch{
	archARM, archARM64, archPPC64LE, archS390X, archX8664,
}

func variantMapX8664toPWSH(arch arch) []variant {
	if arch == archX8664 {
		return []variant{variantPWSH}
	}

	return nil
}

type flavor struct {
	name            string
	compatibleArchs []arch
	variantsMap     func(arch arch) []variant
}

func (f flavor) format(version string, arch arch, variant variant) string {
	if arch == "" {
		return f.formatRunnerTag(version)
	}

	return f.formatHelperTag(version, arch, variant)
}

func (f flavor) formatRunnerTag(version string) string {
	// latest is a special tag
	if f.name == "latest" {
		return "latest"
	}

	return fmt.Sprintf("%s-%s", f.name, version)
}

func (f flavor) formatHelperTag(version string, arch arch, variant variant) string {
	if f.name == "latest" {
		return fmt.Sprintf("%s-latest", arch)
	} else if f.name != "" && variant != "" {
		return fmt.Sprintf("%s-%s-%s-%s", f.name, arch, version, variant)
	} else if f.name == "" && variant != "" {
		return fmt.Sprintf("%s-%s-%s", arch, version, variant)
	} else if f.name == "" && variant == "" {
		return fmt.Sprintf("%s-%s", arch, version)
	}

	return fmt.Sprintf("%s-%s-%s", f.name, arch, version)
}

var runnerFlavors = []flavor{
	{name: "alpine"},
	{name: "alpine3.15"},
	{name: "alpine3.16"},
	{name: "alpine3.17"},
	{name: "alpine3.18"},
	{name: "ubi-fips"},
	{name: "ubuntu"},
	{name: "latest"},
}

var helperFlavors = []flavor{
	{
		name:            "alpine-latest",
		compatibleArchs: supportedArchitectures,
	},
	{
		name:            "latest",
		compatibleArchs: supportedArchitectures,
	},
	{
		name:            "alpine3.15",
		compatibleArchs: supportedArchitectures,
		variantsMap:     variantMapX8664toPWSH,
	},
	{
		name:            "alpine3.16",
		compatibleArchs: supportedArchitectures,
		variantsMap:     variantMapX8664toPWSH,
	},
	{
		name:            "alpine3.17",
		compatibleArchs: supportedArchitectures,
	},
	{
		name:            "alpine3.18",
		compatibleArchs: supportedArchitectures,
	},
	{
		name:            "ubuntu",
		compatibleArchs: supportedArchitectures,
		variantsMap:     variantMapX8664toPWSH,
	},
	{
		name:            "ubi-fips",
		compatibleArchs: []arch{archX8664},
	},
	{
		name:            "",
		compatibleArchs: supportedArchitectures,
		variantsMap: func(arch arch) []variant {
			if arch == archX8664 {
				return []variant{variantServerCore1809, variantServerCore21H2}
			}

			return nil
		},
	},
}

var targetImages = map[image]struct {
	flavors          []flavor
	destinationImage image
}{
	gitlabRunnerImage: {
		flavors:          runnerFlavors,
		destinationImage: gitlabRunnerDestinationImage,
	},
	gitlabRunnerHelperImage: {
		flavors:          helperFlavors,
		destinationImage: gitlabRunnerHelperDestinationImage,
	},
}

type imageSyncPair struct {
	from, to string
}

func newImageSyncPair(sourceRegistry, toRegistry string, fromImg, toImg image, tag string) imageSyncPair {
	return imageSyncPair{
		from: fmt.Sprintf("%s/%s:%s", sourceRegistry, fromImg, tag),
		to:   fmt.Sprintf("%s/%s:%s", toRegistry, toImg, tag),
	}
}

type args struct {
	Version     string   `arg:"--version, required" help:"Version or commit of images to sync e.g. (e.g. v16.0.0 | a54hf6)"`
	Concurrency int      `arg:"--concurrency" help:"The amount of concurrent image pushes to be done" default:"1"`
	Command     []string `arg:"--command" help:"The Command that will be executed to sync the images. Can be multiple strings separated by a space. Default (skopeo)"`
	Images      []image  `arg:"--images" help:"Comma separated list of which types of images to sync - runner, helper. Default: (runner,helper)"`
	Filters     []string `arg:"--filters" help:"Comma separated list of tag filters to be applied to the images to be synced. Empty by default"`
}

func main() {
	args := parseArgs()

	if err := syncImages(args); err != nil {
		log.Fatalln(err)
	}
}

func parseArgs() args {
	var args args

	arg.MustParse(&args)
	if args.Concurrency <= 0 {
		args.Concurrency = 1
	}

	if len(args.Command) == 0 {
		args.Command = []string{"skopeo"}
	}

	if len(args.Images) == 0 {
		args.Images = []image{"runner", "helper"}
	}

	for i, img := range args.Images {
		switch img {
		case "runner":
			args.Images[i] = gitlabRunnerImage
		case "helper":
			args.Images[i] = gitlabRunnerHelperImage
		}
	}

	log.Println(fmt.Sprintf("Will sync images: %+v", args.Images))

	return args
}

func generateTags(filters []string, version string, flavors []flavor) []string {
	var tags []string

	for _, f := range flavors {
		if len(f.compatibleArchs) == 0 {
			tags = append(tags, f.format(version, "", ""))
		}

		for _, arch := range f.compatibleArchs {
			tags = append(tags, f.format(version, arch, ""))

			if f.variantsMap != nil {
				if variants := f.variantsMap(arch); len(variants) > 0 {
					for _, variant := range variants {
						tags = append(tags, f.format(version, arch, variant))
					}
				}
			}
		}
	}

	if len(filters) == 0 {
		return tags
	}

	return lo.Filter(tags, func(tag string, _ int) bool {
		return lo.SomeBy(filters, func(filter string) bool {
			return strings.Contains(tag, filter)
		})
	})
}

func filterTargetRegistries() map[registry]string {
	isEnv := func(env string) bool {
		b, _ := strconv.ParseBool(os.Getenv(env))
		return b
	}

	registries := make(map[registry]string)
	if isEnv(envPushToDockerHub) {
		registries[registryDockerHub] = targetRegistries[registryDockerHub]
	}

	if isEnv(envPushToECR) {
		registries[registryECR] = targetRegistries[registryECR]
	}

	return registries
}

func syncImages(args args) error {
	var images []imageSyncPair

	registries := filterTargetRegistries()
	if len(registries) == 0 {
		log.Printf("Warn: No registries to push to, check the values of %q and %q\n", envPushToDockerHub, envPushToECR)
		return nil
	}

	err := loginRegistries(args, registries)
	if err != nil {
		return err
	}

	for img, target := range targetImages {
		if !lo.Contains(args.Images, img) {
			continue
		}

		tags := generateTags(args.Filters, args.Version, target.flavors)
		for _, registry := range registries {
			for _, tag := range tags {
				images = append(images, newImageSyncPair(sourceRegistry, registry, img, target.destinationImage, tag))
			}
		}
	}

	pool := pool.New().WithErrors().WithMaxGoroutines(args.Concurrency)

	for _, pair := range images {
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

			return cmd.Run()
		})
	}

	return pool.Wait()
}

func loginRegistries(args args, registries map[registry]string) error {
	for registry, addr := range registries {
		log.Printf("Logging into %s:%s", registry, addr)
		switch registry {
		case registryDockerHub:
			if err := loginRegistry(args.Command, addr, os.Getenv(envDockerHubUser), os.Getenv(envDockerHubPassword)); err != nil {
				return err
			}
		case registryECR:
			ecrPassword, err := buildCmdNoStdout(
				"aws",
				"--region",
				"us-east-1",
				"ecr-public",
				"get-login-password",
			).Output()
			if err != nil {
				return fmt.Errorf("getting ecr password for %s: %w", addr, err)
			}

			if err := loginRegistry(args.Command, addr, ecrPublicRegistryUser, string(ecrPassword)); err != nil {
				return err
			}
		default:
		}
	}

	return nil
}

func loginRegistry(command []string, addr string, username, password string) error {
	cmd := buildCmd(
		append(command, []string{
			"login",
			addr,
			"-u",
			username,
			"-p",
			password,
		}...)...,
	)

	return cmd.Run()
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
