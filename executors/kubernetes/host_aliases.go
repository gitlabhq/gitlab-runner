package kubernetes

import (
	"fmt"

	api "k8s.io/api/core/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/services"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
)

type invalidHostAliasDNSError struct {
	service common.Image
	inner   error
	alias   string
}

func (e *invalidHostAliasDNSError) Error() string {
	return fmt.Sprintf(
		"provided host alias %s for service %s is invalid DNS. %s",
		e.alias,
		e.service.Name,
		e.inner,
	)
}

func (e *invalidHostAliasDNSError) Is(err error) bool {
	_, ok := err.(*invalidHostAliasDNSError)
	return ok
}

func createHostAliases(services common.Services, hostAliases []api.HostAlias) ([]api.HostAlias, error) {
	servicesHostAlias, err := createServicesHostAlias(services)
	if err != nil {
		return nil, err
	}

	// The order that we add host aliases matter here. The host file resolves
	// host on a firs-come-first-served basis. We always want to have the
	// service host aliases first so it resolves to that ip.
	var allHostAliases []api.HostAlias
	if servicesHostAlias != nil {
		allHostAliases = append(allHostAliases, *servicesHostAlias)
	}
	allHostAliases = append(allHostAliases, hostAliases...)

	return allHostAliases, nil
}

func createServicesHostAlias(srvs common.Services) (*api.HostAlias, error) {
	var hostnames []string

	for _, srv := range srvs {
		// Services with ports are coming from .gitlab-webide.yml
		// they are used for ports mapping and their aliases are in no way validated
		// so we ignore them. Check out https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1170
		// for details
		if len(srv.Ports) > 0 {
			continue
		}

		serviceMeta := services.SplitNameAndVersion(srv.Name)
		for _, alias := range serviceMeta.Aliases {
			// For backward compatibility reasons a non DNS1123 compliant alias might be generated,
			// this will be removed in https://gitlab.com/gitlab-org/gitlab-runner/issues/6100
			err := dns.ValidateDNS1123Subdomain(alias)
			if err == nil {
				hostnames = append(hostnames, alias)
			}
		}

		for _, alias := range srv.Aliases() {
			err := dns.ValidateDNS1123Subdomain(alias)
			if err != nil {
				return nil, &invalidHostAliasDNSError{service: srv, inner: err, alias: alias}
			}

			hostnames = append(hostnames, alias)
		}
	}

	// no service hostnames to add to aliases
	if len(hostnames) == 0 {
		return nil, nil
	}

	return &api.HostAlias{IP: "127.0.0.1", Hostnames: hostnames}, nil
}
