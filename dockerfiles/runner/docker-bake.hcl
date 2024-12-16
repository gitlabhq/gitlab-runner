variable "RUNNER_IMAGES_REGISTRY" {
  default = "registry.gitlab.com/gitlab-org/ci-cd/runner-tools/base-images"
}

variable "RUNNER_IMAGES_VERSION" {
  default = "0.0.0"
}

target "base" {
  contexts = {
    binary_dir = "../../out/binaries/"
  }

  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/s390x",
    "linux/ppc64le",
  ]
}

target "ubuntu" {
  inherits = ["base"]

  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner:${RUNNER_IMAGES_VERSION}-ubuntu"
  }
  output = ["type=oci,dest=./../../out/runner-images/ubuntu.tar,tar=true"]
}

target "ubi-fips" {
  inherits = ["base"]

  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner:${RUNNER_IMAGES_VERSION}-ubi-fips"
  }

  platforms = ["linux/amd64"]
  output    = ["type=oci,dest=./../../out/runner-images/ubi-fips.tar,tar=true"]
}

target "alpine" {
  inherits = ["base"]

  name = "alpine-${replace(version, ".", "-")}"

  matrix = {
    version = ["latest", "3.18", "3.19", "3.21"]
  }

  args = {
    BASE_IMAGE = "${RUNNER_IMAGES_REGISTRY}/runner:${RUNNER_IMAGES_VERSION}-alpine-${version}"
  }
  output = ["type=oci,dest=./../../out/runner-images/alpine-${version}.tar,tar=true"]
}

group "all" {
  targets = [
    "ubuntu",
    "alpine",
    "ubi-fips",
  ]
}
