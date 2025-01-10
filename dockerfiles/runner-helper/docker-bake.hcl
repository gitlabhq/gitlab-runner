variable "RUNNER_IMAGES_REGISTRY" {
  default = "registry.gitlab.com/gitlab-org/ci-cd/runner-tools/base-images"
}

variable "RUNNER_IMAGES_VERSION" {
  default = "0.0.0"
}

variable "HOST_ARCH" {
  default = "amd64"
}

variable "HOST_FLAVOR" {
  default = "alpine-3.21"
}

common-platforms = [
  "linux/amd64",
  "linux/arm",
  "linux/arm64",
  "linux/s390x",
  "linux/ppc64le",
  "linux/riscv64"
]

alpine-platforms = {
  "3.18" : setsubtract(common-platforms, ["linux/riscv64"]),
  "3.19" : setsubtract(common-platforms, ["linux/riscv64"]),
  "3.21" : common-platforms,
  "latest" : common-platforms,
  "edge" : common-platforms,
}


target "base" {
  contexts = {
    binary_dir = "../../out/binaries/gitlab-runner-helper"
  }
}

target "alpine" {
  inherits = ["base"]
  
  name = "alpine-${replace(v.version, ".", "-")}-${v.arch}"

  matrix = {
    v = flatten([
      for key, values in alpine-platforms : [
        for plat in values : { version: key, arch: split("/", plat)[1] }
      ]
    ])
  }

  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-alpine-${v.version}"
  }

  platforms = ["linux/${v.arch}"]
  output    = ["type=oci,dest=./../../out/helper-images/alpine${v.version == "latest" || v.version == "edge" ? "-${v.version}" : v.version}-${v.arch == "amd64" ? "x86_64" : v.arch}.tar"]
}

target "alpine-pwsh" {
  inherits = ["base"]

  name = "alpine-${replace(version, ".", "-")}-pwsh"

  matrix = {
    version = keys(alpine-platforms)
  }

  platforms = ["linux/amd64"]
  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-alpine-${version}-pwsh"
  }
  output = ["type=oci,dest=./../../out/helper-images/alpine${version == "latest" || version == "edge" ? "-${version}" : version}-x86_64-pwsh.tar,tar=true"]
}

target "ubuntu" {
  inherits = ["base"]

  name = "ubuntu-${replace(platform, "/", "-")}"

  matrix = {
    platform = common-platforms
  }

  platforms = [platform]
  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-ubuntu"
  }

  output = ["type=oci,dest=./../../out/helper-images/ubuntu-${split("/", platform)[1] == "amd64" ? "x86_64" : split("/", platform)[1]}.tar,tar=true"]
}

target "ubuntu-pwsh" {
  inherits = ["base"]

  platforms = ["linux/amd64"]
  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-ubuntu-pwsh"
  }

  output = ["type=oci,dest=./../../out/helper-images/ubuntu-x86_64-pwsh.tar,tar=true"]
}

target "ubi-fips" {
  inherits = ["base"]

  platforms = ["linux/amd64"]
  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-ubi-fips"
    SRC_SUFFIX = "-fips"
  }

  output = ["type=oci,dest=./../../out/helper-images/ubi-fips-x86_64.tar,tar=true"]
}

target "windows" {
  inherits = ["base"]

  name = "windows-${replace(version, ":", "-")}"

  matrix = {
    version = ["nanoserver:ltsc2019", "nanoserver:ltsc2022", "servercore:ltsc2019", "servercore:ltsc2022"]
  }

  platforms = ["windows/amd64"]
  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-${replace(version, ":", "-")}"
    SRC_SUFFIX = ".exe"
    DST_SUFFIX = ".exe"
  }

  output = ["type=oci,dest=./../../out/helper-images/windows-${replace(version, ":", "-")}.tar,tar=true"]
}

# Used for local testing, creates the gitlab-runner-helper:local image in the user's current docker context
target "host-image" {
  inherits = ["base"]

  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner-helper:${RUNNER_IMAGES_VERSION}-${HOST_FLAVOR}"
  }

  platforms = ["linux/${HOST_ARCH}"]
  output    = ["type=docker"]
  tags      = ["gitlab-runner-helper:local"]
}

group "all" {
  targets = [
    "alpine",
    "alpine-pwsh",
    "ubuntu",
    "ubuntu-pwsh",
    "ubi-fips",
    "windows",
  ]
}
