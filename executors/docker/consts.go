package docker

import "time"

const dockerLabelPrefix = "com.gitlab.gitlab-runner"

const prebuiltImageName = "gitlab/gitlab-runner-helper"
const prebuiltImageExtension = ".tar.xz"

const dockerCleanupTimeout = 5 * time.Minute

const waitForContainerTimeout = 15 * time.Second

const osTypeLinux = "linux"
const osTypeWindows = "windows"

const metadataOSType = "OSType"
