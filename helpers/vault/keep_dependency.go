package vault

import (
	// dummy import of Vault's API library to make `go mod tidy`
	// and `make check_modules` happy. Will be removed when proper implementation
	// will be added (https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2373)!
	_ "github.com/hashicorp/vault/api"
)
