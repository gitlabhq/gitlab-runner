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
}

func (e *invalidHostAliasDNSError) Error() string {
	return fmt.Sprintf(
		"provided host alias %s for service %s is invalid DNS. %s",
		e.service.Alias,
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

	var allHostAliases []api.HostAlias
	if servicesHostAlias != nil {
		allHostAliases = append(allHostAliases, *servicesHostAlias)
	}
	allHostAliases = append(allHostAliases, hostAliases...)

	return allHostAliases, nil
}

func createServicesHostAlias(srvs common.Services) (*api.HostAlias, error) {
	servicesHostAlias := api.HostAlias{IP: "127.0.0.1"}

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
				servicesHostAlias.Hostnames = append(servicesHostAlias.Hostnames, alias)
			}
		}

		if srv.Alias == "" {
			continue
		}

		err := dns.ValidateDNS1123Subdomain(srv.Alias)
		if err != nil {
			return nil, &invalidHostAliasDNSError{service: srv, inner: err}
		}

		servicesHostAlias.Hostnames = append(servicesHostAlias.Hostnames, srv.Alias)
	}

	return &servicesHostAlias, nil
}
