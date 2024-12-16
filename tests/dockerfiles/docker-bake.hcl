variable "CI_REGISTRY_IMAGE" {
  default = "local"
}

target "tests-images" {
  name = image.name
  
  matrix = {
    image = [
      { name = "alpine-no-root", dockerfile = "alpine-no-root/Dockerfile" },
      { name = "alpine-entrypoint", dockerfile = "alpine-entrypoint/Dockerfile" },
      { name = "alpine-entrypoint-stderr", dockerfile = "alpine-entrypoint/Dockerfile.stderr" },
      { name = "alpine-entrypoint-pre-post-trap", dockerfile = "alpine-entrypoint/Dockerfile.pre-post-trap" },
      { name = "powershell-entrypoint-pre-post-trap", dockerfile = "powershell-entrypoint/Dockerfile.pre-post-trap" },
      { name = "alpine-id-overflow", dockerfile = "alpine-id-overflow/Dockerfile" },
      { name = "helper-entrypoint", dockerfile = "gitlab-runner-helper-entrypoint/dockerfile" },
    ]
  }

  dockerfile = image.dockerfile
  contexts = {
    binary_dir = "../../out/binaries/gitlab-runner-helper/"
  }

  tags = ["${CI_REGISTRY_IMAGE}/${image.name}:latest"]
}
