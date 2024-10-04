# Test Images

These images are used for integration or unit tests for Kubernetes or Docker.

## Structure

The directories in `tests/dockerfiles` represent images hosted in the GitLab Runner Docker Registry.

For example, the `alpine-id-overflow` image is located at `registry.gitlab.com/gitlab-org/gitlab-runner/alpine-id-overflow:latest`.

Newer images might be located at `registry.gitlab.com/gitlab-org/gitlab-runner/test/alpine-id-overflow:latest`. It's recommended that this path be used.

## Versioning

Some of these images are tagged only with their latest tag, while others are tagged with a specific version.

When rebuilding an image, increment the version and push the new image. This ensures we don't overwrite images that are in use or might be useful for debugging.

For images without versions, it's recommended to start from `v1`.

## Building

First, create a GitLab token and authenticate your Docker daemon with it.

Then, in the image directory, run:

```bash
docker build -t registry.gitlab.com/gitlab-org/gitlab-runner/alpine-id-overflow:v1 .
docker push registry.gitlab.com/gitlab-org/gitlab-runner/alpine-id-overflow:v1
```