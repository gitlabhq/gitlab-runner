package packages

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/magefile/mage/sh"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
)

type Type string

const (
	Deb     Type = "deb"
	Rpm     Type = "rpm"
	RpmFips Type = "rpm-fips"

	HelperImagesPackage = build.AppName + "-helper-images"
)

// Create creates a package based on the type
func Create(blueprint Blueprint) error {
	var opts []string
	switch blueprint.Data().pkgType {
	case Deb:
		opts = []string{
			"--depends", "ca-certificates",
			"--category", "admin",
			"--deb-priority", "optional",
			"--deb-compression", "bzip2",
			"--deb-suggests", "docker-engine",
		}
	case Rpm:
		opts = []string{
			"--rpm-compression", "bzip2",
			"--rpm-os", "linux",
			"--rpm-digest", "sha256",
			"--conflicts", build.AppName + "-fips",
		}
	case RpmFips:
		opts = []string{
			"--rpm-compression", "bzip2",
			"--rpm-os", "linux",
			"--depends", "openssl",
			"--rpm-digest", "sha256",
			"--conflicts", build.AppName,
		}
	}

	if err := createPackage(blueprint, opts); err != nil {
		return err
	}

	return signPackage(blueprint)
}

func createPackage(blueprint Blueprint, opts []string) error {
	p := blueprint.Data()

	if err := os.MkdirAll(fmt.Sprintf("out/%s", p.pkgType), 0700); err != nil {
		return err
	}

	if Type(p.postfix) != "-fips" {
		fullVersion := build.Version() + "-" + blueprint.Env().Value(iteration)
		opts = append(opts, "--depends", HelperImagesPackage+" = "+fullVersion)
	}

	pkgName := build.AppName

	args := append(opts, []string{ //nolint:gocritic
		"--verbose",
		"--package", p.pkgFile,
		"--force",
		"--iteration", blueprint.Env().Value(iteration),
		"--input-type", "dir",
		"--output-type", string(p.pkgType),
		"--name", pkgName + p.postfix,
		"--description", "GitLab Runner",
		"--version", build.Version(),
		"--url", "https://gitlab.com/gitlab-org/gitlab-runner",
		"--maintainer", "GitLab Inc. <support@gitlab.com>",
		"--license", "MIT",
		"--vendor", "GitLab Inc.",
		"--architecture", p.packageArch,
		"--depends", "git",
		"--depends", "curl",
		"--depends", "tar",
		"--after-install", "packaging/scripts/postinst." + string(p.pkgType),
		"--before-remove", "packaging/scripts/prerm." + string(p.pkgType),
		"--conflicts", pkgName + "-beta",
		"--conflicts", "gitlab-ci-multi-runner",
		"--conflicts", "gitlab-ci-multi-runner-beta",
		"--provides", "gitlab-ci-multi-runner",
		"--replaces", "gitlab-ci-multi-runner",
		"packaging/root/=/",
		fmt.Sprintf("%s=/usr/bin/gitlab-runner", p.runnerBinary),
	}...)

	args = append(args, p.prebuiltImages...)

	err := sh.RunV("fpm", args...)
	if err != nil {
		return fmt.Errorf("failed to create %s package: %w", p.pkgType, err)
	}

	return nil
}

func CreateHelper(blueprint Blueprint) error {
	var opts []string
	switch blueprint.Data().pkgType {
	case Deb:
		opts = []string{
			"--category", "admin",
			"--deb-priority", "optional",
			"--deb-compression", "bzip2",
		}
	case Rpm:
		opts = []string{
			"--rpm-compression", "bzip2",
			"--rpm-os", "linux",
			"--rpm-digest", "sha256",
		}
	}

	if err := createHelperImagesPackage(blueprint, opts); err != nil {
		return err
	}

	return signPackage(blueprint)
}

func createHelperImagesPackage(blueprint Blueprint, opts []string) error {
	p := blueprint.Data()

	if err := os.MkdirAll(fmt.Sprintf("out/%s", p.pkgType), 0700); err != nil {
		return err
	}

	pkgName := HelperImagesPackage

	args := append(opts, []string{ //nolint:gocritic
		"--verbose",
		"--package", p.pkgFile,
		"--force",
		"--iteration", blueprint.Env().Value(iteration),
		"--input-type", "dir",
		"--output-type", string(p.pkgType),
		"--name", pkgName,
		"--description", "GitLab Runner Helper Docker Images",
		"--version", build.Version(),
		"--url", "https://gitlab.com/gitlab-org/gitlab-runner",
		"--maintainer", "GitLab Inc. <support@gitlab.com>",
		"--license", "MIT",
		"--vendor", "GitLab Inc.",
		"--architecture", "noarch",
	}...)

	// fix https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38394 for deb packages at least...
	if p.pkgType == Deb {
		args = append(args,
			"--provides", pkgName,
			"--replaces", build.AppName)
	}

	args = append(args, p.prebuiltImages...)

	err := sh.RunV("fpm", args...)
	if err != nil {
		return fmt.Errorf("failed to create %s package: %w", p.pkgType, err)
	}

	return nil
}

func signPackage(blueprint Blueprint) error {
	gpgKey := blueprint.Env().Value(gPGKeyID)
	if gpgKey == "" {
		fmt.Println("gpg key is empty, skipping signing")
		return nil
	}

	gpgPass := blueprint.Env().Value(gPGPassphrase)
	if gpgPass == "" {
		return fmt.Errorf("gpg passphrase is empty")
	}

	var err error
	switch blueprint.Data().pkgType {
	case Deb:
		err = sh.RunV("dpkg-sig",
			"-g", fmt.Sprintf("--no-tty --digest-algo 'sha512' --passphrase '%s' --pinentry-mode=loopback", gpgPass),
			"-k", gpgKey,
			"--sign", "builder",
			blueprint.Data().pkgFile,
		)
	case Rpm, RpmFips:
		command := []string{
			"echo yes | setsid rpm",
			"--define", strconv.Quote(fmt.Sprintf("_gpg_name %s", gpgKey)),
			"--define", strconv.Quote("_signature gpg"),
			"--define", strconv.Quote("__gpg_check_password_cmd /bin/true"),
			"--define", strconv.Quote(fmt.Sprintf("__gpg_sign_cmd $(command -v gpg) --batch --no-armor --digest-algo 'sha512' --passphrase '%s' --pinentry-mode=loopback --no-secmem-warning -u '%s' --sign --detach-sign --output %%{__signature_filename} %%{__plaintext_filename}", gpgPass, gpgKey)),
			"--addsign", blueprint.Data().pkgFile,
		}

		err = sh.RunV("sh", "-c", strings.Join(command, " "))
	}

	if err != nil {
		return fmt.Errorf("failed to sign %s package: %w", blueprint.Data().pkgType, err)
	}

	return nil
}
