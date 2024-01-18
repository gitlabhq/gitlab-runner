package images

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

var helperImageName = env.NewDefault("HELPER_IMAGE_NAME", "gitlab-runner-helper")

var platformMap = map[string]string{
	"x86_64":  "linux/amd64",
	"arm":     "linux/arm/v7",
	"arm64":   "linux/arm64/v8",
	"s390x":   "linux/s390x",
	"ppc64le": "linux/ppc64le",
	"riscv64": "linux/riscv64",
}

var flavorsSupportingPWSH = []string{
	"alpine",
	"alpine3.16",
	"alpine3.17",
	"alpine3.18",
	"ubuntu",
}

type helperBuild struct {
	archive  string
	platform string
	tags     helperTagsList
}

type helperTagsList struct {
	suffix        string
	baseTemplate  string
	prefix        string
	arch          string
	imageName     string
	registryImage string
	isLatest      bool
}

func newHelperTagsList(prefix, suffix, arch, imageName, registryImage string, isLatest bool) helperTagsList {
	return helperTagsList{
		prefix:        prefix,
		suffix:        suffix,
		arch:          arch,
		registryImage: registryImage,
		imageName:     imageName,
		isLatest:      isLatest,
		baseTemplate:  "{{ .RegistryImage }}/{{ .ImageName }}:{{ .Prefix }}{{ .Arch}}-"}
}

func (l helperTagsList) render(raw string) string {
	context := struct {
		RegistryImage string
		ImageName     string
		Prefix        string
		Arch          string
		Revision      string
		RefTag        string
	}{
		RegistryImage: l.registryImage,
		ImageName:     l.imageName,
		Prefix:        l.prefix,
		Arch:          l.arch,
		Revision:      build.Revision(),
		RefTag:        build.RefTag(),
	}

	var out bytes.Buffer
	tmpl := lo.Must(template.New("tmpl").Parse(l.baseTemplate + raw + l.suffix))

	lo.Must0(tmpl.Execute(&out, &context))

	return out.String()
}

func (l helperTagsList) revisionTag() string {
	return l.render("{{ .Revision }}")
}

func (l helperTagsList) versionTag() string {
	return l.render("{{ .RefTag }}")
}

func (l helperTagsList) latestTag() (string, bool) {
	return l.render("latest"), l.isLatest
}

func (l helperTagsList) all() []string {
	all := []string{
		l.revisionTag(),
		l.versionTag(),
	}
	if latest, isLatest := l.latestTag(); isLatest {
		all = append(all, latest)
	}

	return all
}

type helperBlueprintImpl struct {
	build.BlueprintBase

	data []helperBuild
}

func (b helperBlueprintImpl) Dependencies() []build.Component {
	return lo.Map(b.data, func(item helperBuild, _ int) build.Component {
		return build.NewDockerImageArchive(item.archive)
	})
}

func (b helperBlueprintImpl) Artifacts() []build.Component {
	return lo.Flatten(lo.Map(b.data, func(item helperBuild, _ int) []build.Component {
		return lo.Map(item.tags.all(), func(item string, _ int) build.Component {
			return build.NewDockerImage(item)
		})
	}))
}

func (b helperBlueprintImpl) Data() []helperBuild {
	return b.data
}

func AssembleReleaseHelper(flavor, prefix string) build.TargetBlueprint[build.Component, build.Component, []helperBuild] {
	var archs []string
	switch flavor {
	case "ubi-fips":
		archs = []string{"x86_64"}
	case "alpine-edge":
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le", "riscv64"}
	default:
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le"}
	}

	builds := helperBlueprintImpl{
		BlueprintBase: build.NewBlueprintBase(ci.RegistryImage, ci.RegistryAuthBundle, docker.BuilderEnvBundle, helperImageName),
		data:          []helperBuild{},
	}

	imageName := builds.Env().Value(helperImageName)
	registryImage := builds.Env().Value(ci.RegistryImage)

	for _, arch := range archs {
		builds.data = append(builds.data, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-%s.tar.xz", flavor, arch),
			platform: platformMap[arch],
			tags:     newHelperTagsList(prefix, "", arch, imageName, registryImage, build.IsLatest()),
		})
	}

	if lo.Contains(flavorsSupportingPWSH, flavor) {
		builds.data = append(builds.data, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-x86_64-pwsh.tar.xz", flavor),
			platform: platformMap["x86_64"],
			tags:     newHelperTagsList(prefix, "-pwsh", "x86_64", imageName, registryImage, build.IsLatest()),
		})
	}

	return builds
}

func ReleaseHelper(blueprint build.TargetBlueprint[build.Component, build.Component, []helperBuild], publish bool) error {
	env := blueprint.Env()
	builder := docker.NewBuilder(
		env.Value(docker.Host),
		env.Value(docker.CertPath),
	)

	logout, err := builder.Login(
		env.Value(ci.RegistryUser),
		env.Value(ci.RegistryPassword),
		env.Value(ci.Registry),
	)
	if err != nil {
		return err
	}
	defer logout()

	for _, build := range blueprint.Data() {
		if err := releaseImageTags(
			builder,
			build,
			publish,
		); err != nil {
			return err
		}
	}

	return nil
}

func releaseImageTags(builder *docker.Builder, build helperBuild, publish bool) error {
	baseTag := build.tags.revisionTag()
	versionTag := build.tags.versionTag()
	latestTag, isLatest := build.tags.latestTag()

	if err := builder.Import(build.archive, baseTag, build.platform); err != nil {
		return err
	}

	if err := builder.Tag(baseTag, versionTag); err != nil {
		return err
	}

	if isLatest {
		if err := builder.Tag(baseTag, latestTag); err != nil {
			return err
		}
	}

	if !publish {
		return nil
	}

	tagsToPush := []string{baseTag, versionTag}
	if isLatest {
		tagsToPush = append(tagsToPush, latestTag)
	}

	for _, tag := range tagsToPush {
		if err := builder.Push(tag); err != nil {
			return err
		}
	}

	return nil
}
