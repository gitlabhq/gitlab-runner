package images

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

const (
	DefaultFlavor = "ubuntu"
	DefaultArchs  = "amd64"

	runnerHomeDir = "dockerfiles/runner"
)

var (
	runnerImageName = env.NewDefault("RUNNER_IMAGE_NAME", "")

	dockerMachineVersion       = env.NewDefault("DOCKER_MACHINE_VERSION", "v0.16.2-gitlab.27")
	dockerMachineAmd64Checksum = env.NewDefault("DOCKER_MACHINE_AMD64_CHECKSUM", "37a04553079f97d7540033426cbeb9075a70758d39234fe14a896c0354f4acca")
	dockerMachineArm64Checksum = env.NewDefault("DOCKER_MACHINE_ARM64_CHECKSUM", "97f82859f3ca74a9a821477210393d90d128a2f1e1edad383509864dbe34ef41")
	// s390x and ppc64le are not being released
	dockerMachineS390xChecksum   = env.New("DOCKER_MACHINE_S390X_CHECKSUM")
	dockerMachinePpc64leChecksum = env.New("DOCKER_MACHINE_PPC64LE_CHECKSUM")

	dumbInitVersion         = env.NewDefault("DUMB_INIT_VERSION", "1.2.2")
	dumbInitAmd64Checksum   = env.NewDefault("DUMB_INIT_AMD64_CHECKSUM", "37f2c1f0372a45554f1b89924fbb134fc24c3756efaedf11e07f599494e0eff9")
	dumbInitArm64Checksum   = env.NewDefault("DUMB_INIT_ARM64_CHECKSUM", "45b1bbf56cc03edda81e4220535a025bfe3ed6e93562222b9be4471005b3eeb3")
	dumbInitS390xChecksum   = env.NewDefault("DUMB_INIT_S390X_CHECKSUM", "8b3808c3c06d008b8f2eeb2789c7c99e0450b678d94fb50fd446b8f6a22e3a9d")
	dumbInitPpc64leChecksum = env.NewDefault("DUMB_INIT_PPC64LE_CHECKSUM", "88b02a3bd014e4c30d8d54389597adc4f5a36d1d6b49200b5a4f6a71026c2246")

	gitLfsVersion = env.NewDefault("GIT_LFS_VERSION", "3.4.0")

	ubuntuVersion    = env.NewDefault("UBUNTU_VERSION", "20.04")
	alpine316Version = env.NewDefault("ALPINE_316_VERSION", "3.16.5")
	alpine317Version = env.NewDefault("ALPINE_317_VERSION", "3.17.3")
	alpine318Version = env.NewDefault("ALPINE_318_VERSION", "3.18.2")
	alpine319Version = env.NewDefault("ALPINE_319_VERSION", "3.19.0")

	ubiFIPSBaseImage    = env.NewDefault("UBI_FIPS_BASE_IMAGE", "registry.gitlab.com/gitlab-org/gitlab-runner/ubi-fips-base")
	ubiFIPSVersion      = env.NewDefault("UBI_FIPS_VERSION", "9.4-13")
	ubiMinimalImage     = env.NewDefault("UBI_MINIMAL_IMAGE", "redhat/ubi9-minimal")
	ubiMinimalVersion   = env.NewDefault("UBI_MINIMAL_VERSION", "9.4-1194")

	buildxRetry = env.NewDefault("RUNNER_IMAGES_DOCKER_BUILDX_RETRY", "0")
)

var checksumsFiles = map[string]string{
	"DOCKER_MACHINE": "/usr/bin/docker-machine",
	"DUMB_INIT":      "/usr/bin/dumb-init",
}

var flavorAliases = map[string][]string{
	"alpine3.19": {"alpine", "alpine3.19"},
}

type buildRunnerParams struct {
	flavor string
	archs  []string
}

type runnerBlueprintImpl struct {
	build.BlueprintBase

	dependencies []runnerImageFileDependency
	artifacts    []string
	params       buildRunnerParams
}

type runnerImageFileDependency struct {
	build.Component

	destination string
}

func (r runnerBlueprintImpl) Dependencies() []runnerImageFileDependency {
	return r.dependencies
}

func (r runnerBlueprintImpl) Artifacts() []build.Component {
	return lo.Map(r.artifacts, func(item string, _ int) build.Component {
		return build.NewDockerImage(item)
	})
}

func (r runnerBlueprintImpl) Data() buildRunnerParams {
	return r.params
}

func AssembleBuildRunner(flavor, targetArchs string) build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams] {
	archs := strings.Split(strings.ToLower(targetArchs), " ")

	flavors := flavorAliases[flavor]
	if len(flavors) == 0 {
		flavors = []string{flavor}
	}

	base := build.NewBlueprintBase(
		ci.RegistryImage,
		ci.RegistryAuthBundle,
		docker.BuilderEnvBundle,
		runnerImageName,
		dockerMachineVersion,
		dumbInitVersion,
		gitLfsVersion,
		ubuntuVersion,
		alpine316Version,
		alpine317Version,
		alpine318Version,
		alpine319Version,
		ubiFIPSBaseImage,
		ubiFIPSVersion,
		ubiMinimalImage,
		ubiMinimalVersion,
		dockerMachineAmd64Checksum,
		dockerMachineArm64Checksum,
		dockerMachineS390xChecksum,
		dockerMachinePpc64leChecksum,
		dumbInitAmd64Checksum,
		dumbInitArm64Checksum,
		dumbInitS390xChecksum,
		dumbInitPpc64leChecksum,
		buildxRetry,
	)

	return runnerBlueprintImpl{
		BlueprintBase: base,
		dependencies:  assembleDependencies(flavor, archs),
		artifacts: tags(
			flavors,
			base.Env().Value(ci.RegistryImage),
			base.Env().Value(runnerImageName),
			build.RefTag(),
		),
		params: buildRunnerParams{
			flavor: flavor,
			archs:  archs,
		},
	}
}

func BuildRunner(blueprint build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams], publish bool) error {
	flavor := blueprint.Data().flavor
	archs := blueprint.Data().archs

	platform := flavor
	if strings.HasPrefix(platform, "alpine") {
		platform = "alpine"
	}

	if err := writeChecksums(archs, blueprint.Env()); err != nil {
		return fmt.Errorf("writing checksums: %w", err)
	}

	if err := copyDependencies(blueprint.Dependencies()); err != nil {
		return fmt.Errorf("copying dependencies: %w", err)
	}

	baseImagesFlavor := map[string]string{
		"ubuntu":        fmt.Sprintf("ubuntu:%s", blueprint.Env().Value(ubuntuVersion)),
		"alpine3.16":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine316Version)),
		"alpine3.17":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine317Version)),
		"alpine3.18":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine318Version)),
		"alpine3.19":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine319Version)),
		"alpine-latest": "alpine:latest",
		"ubi-fips": fmt.Sprintf(
			"%s:%s",
			blueprint.Env().Value(ubiFIPSBaseImage),
			blueprint.Env().Value(ubiFIPSVersion),
		),
	}

	contextPath := filepath.Join(runnerHomeDir, platform)
	baseImage := baseImagesFlavor[flavor]

	return buildx(
		contextPath,
		baseImage,
		blueprint,
		publish,
	)
}

func writeChecksums(archs []string, env build.BlueprintEnv) error {
	checksumBinaries := map[string][]string{}
	checksums := map[string]string{}
	for _, v := range env.All() {
		value := env.Value(v)
		if value == "" || !strings.HasSuffix(v.Key, "_CHECKSUM") {
			continue
		}

		split := strings.Split(v.Key, "_")
		binaryName := strings.Join(split[:len(split)-2], "_")
		arch := strings.ToLower(split[len(split)-2])
		checksumBinaries[binaryName] = append(checksumBinaries[binaryName], arch)
		checksums[binaryName+"_"+arch] = value
	}

	for _, arch := range archs {
		var sb strings.Builder
		for binary, checksumArchs := range checksumBinaries {
			if !lo.Contains(checksumArchs, arch) {
				continue
			}

			checksumFile := checksumsFiles[binary]
			checksum := checksums[binary+"_"+arch]

			sb.WriteString(fmt.Sprintf("%s  %s\n", checksum, checksumFile))
		}

		checksumsFile := sb.String()
		fmt.Printf("Writing checksums for %s: \n%s", arch, checksumsFile)

		err := os.WriteFile(
			filepath.Join(runnerHomeDir, fmt.Sprintf("checksums-%s", arch)),
			[]byte(checksumsFile),
			0600,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyDependencies(deps []runnerImageFileDependency) error {
	for _, dep := range deps {
		from := dep.Value()
		to := dep.destination
		if err := sh.RunV("cp", from, to); err != nil {
			return fmt.Errorf("copying %s to %s: %w", from, to, err)
		}
	}

	return nil
}

func assembleDependencies(flavor string, archs []string) []runnerImageFileDependency {
	installDeps := []string{
		filepath.Join(runnerHomeDir, "install-deps"),
	}

	copyMap := map[string][]string{
		"ubuntu":   installDeps,
		"alpine":   installDeps,
		"ubi-fips": installDeps,
	}

	for _, arch := range archs {
		debArch := arch
		if arch == "ppc64le" {
			debArch = "ppc64el"
		}

		checksumsFile := filepath.Join(runnerHomeDir, fmt.Sprintf("checksums-%s", arch))

		copyMap["ubuntu"] = append(
			copyMap["ubuntu"],
			checksumsFile,
			fmt.Sprintf("out/deb/gitlab-runner_%s.deb", debArch),
		)

		copyMap["alpine"] = append(
			copyMap["alpine"],
			checksumsFile,
			fmt.Sprintf("out/binaries/gitlab-runner-linux-%s", arch),
		)

		if flavor == "ubi-fips" && arch == "amd64" {
			copyMap["ubi-fips"] = append(
				copyMap["ubi-fips"],
				checksumsFile,
				fmt.Sprintf("out/binaries/gitlab-runner-linux-%s-fips", arch),
				fmt.Sprintf("out/rpm/gitlab-runner_%s-fips.rpm", arch),
			)
		}
	}

	var dependencies []runnerImageFileDependency

	for to, fromFiles := range copyMap {
		for _, from := range fromFiles {
			dependencies = append(dependencies, runnerImageFileDependency{
				Component:   build.NewFile(from),
				destination: filepath.Join(runnerHomeDir, to, path.Base(from)),
			})
		}
	}

	return dependencies
}

func buildx(
	contextPath, baseImage string,
	blueprint build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams],
	publish bool,
) error {
	args, err := buildxArgs(contextPath, baseImage, blueprint, publish)
	if err != nil {
		return err
	}

	env := blueprint.Env()
	builder := docker.NewBuilder(
		env.Value(docker.Host),
		env.Value(docker.CertPath),
	)
	defer func() {
		_ = builder.CleanupContext()
	}()

	if err := builder.SetupContext(); err != nil {
		return err
	}

	if publish {
		logout, err := builder.Login(
			env.Value(ci.RegistryUser),
			env.Value(ci.RegistryPassword),
			env.Value(ci.Registry),
		)
		if err != nil {
			return err
		}

		defer logout()
	}

	fmt.Printf("Buildx builder will retry failed builds %d times\n", env.Int(buildxRetry))
	return retry.NewNoValue(
		retry.New().WithMaxTries(env.Int(buildxRetry)).WithStdout(),
		func() error {
			return builder.Buildx(append([]string{"build"}, args...)...)
		},
	).Run()
}

func buildxArgs(
	contextPath, baseImage string,
	blueprint build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams],
	publish bool,
) ([]string, error) {
	env := blueprint.Env()
	args := []string{
		"--build-arg", fmt.Sprintf("BASE_IMAGE=%s", baseImage),
		"--build-arg", fmt.Sprintf("UBI_MINIMAL_IMAGE=%s", fmt.Sprintf("%s:%s", env.Value(ubiMinimalImage), env.Value(ubiMinimalVersion))),
		"--build-arg", fmt.Sprintf("DOCKER_MACHINE_VERSION=%s", env.Value(dockerMachineVersion)),
		"--build-arg", fmt.Sprintf("DUMB_INIT_VERSION=%s", env.Value(dumbInitVersion)),
		"--build-arg", fmt.Sprintf("GIT_LFS_VERSION=%s", env.Value(gitLfsVersion)),
	}
	args = append(args, lo.Map(blueprint.Artifacts(), func(tag build.Component, _ int) string {
		return fmt.Sprintf("--tag=%s", tag.Value())
	})...)

	dockerOS, err := sh.Output("docker", "version", "-f", "{{.Server.Os}}")
	if err != nil {
		return nil, err
	}
	args = append(args, lo.Map(blueprint.Data().archs, func(arch string, _ int) string {
		return fmt.Sprintf("--platform=%s/%s", dockerOS, arch)
	})...)

	if publish {
		args = append(args, "--push")
	} else if len(blueprint.Data().archs) == 1 {
		args = append(args, "--load")
	}

	args = append(args, contextPath)

	return args, nil
}

func tags(baseImages []string, registryImage, imageName, refTag string) []string {
	var tags []string

	image := registryImage
	if imageName != "" {
		image = fmt.Sprintf("%s/%s", registryImage, imageName)
	}

	for _, base := range baseImages {
		tags = append(tags,
			fmt.Sprintf("%s:%s-%s", image, base, refTag),
			fmt.Sprintf("%s:%s-%s", image, base, build.Revision()),
		)
		if base == DefaultFlavor {
			tags = append(tags, fmt.Sprintf("%s:%s", image, refTag))
		}

		if build.IsLatest() {
			tags = append(tags, fmt.Sprintf("%s:%s", image, base))
			if base == DefaultFlavor {
				tags = append(tags, fmt.Sprintf("%s:latest", image))
			}
		}
	}

	return tags
}
