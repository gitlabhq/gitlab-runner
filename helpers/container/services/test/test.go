package test

type ServiceDescription struct {
	Description string
	Image       string
	Service     string
	Version     string
	Alias       string
	Alternative string
}

// Services is an array of test service descriptions representing different possibilities of names/identifiers
var Services = []ServiceDescription{
	{"service", "service:latest", "service", "latest", "service", ""},
	{"service:version", "service:version", "service", "version", "service", ""},
	{"namespace/service", "namespace/service:latest", "namespace/service", "latest", "namespace__service", "namespace-service"},
	{"namespace/service:version", "namespace/service:version", "namespace/service", "version", "namespace__service", "namespace-service"},
	{"domain.tld/service", "domain.tld/service:latest", "domain.tld/service", "latest", "domain.tld__service", "domain.tld-service"},
	{"domain.tld/service:version", "domain.tld/service:version", "domain.tld/service", "version", "domain.tld__service", "domain.tld-service"},
	{"domain.tld/namespace/service", "domain.tld/namespace/service:latest", "domain.tld/namespace/service", "latest", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld/namespace/service:version", "domain.tld/namespace/service:version", "domain.tld/namespace/service", "version", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld:8080/service", "domain.tld:8080/service:latest", "domain.tld/service", "latest", "domain.tld__service", "domain.tld-service"},
	{"domain.tld:8080/service:version", "domain.tld:8080/service:version", "domain.tld/service", "version", "domain.tld__service", "domain.tld-service"},
	{"domain.tld:8080/namespace/service", "domain.tld:8080/namespace/service:latest", "domain.tld/namespace/service", "latest", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld:8080/namespace/service:version", "domain.tld:8080/namespace/service:version", "domain.tld/namespace/service", "version", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"subdomain.domain.tld:8080/service", "subdomain.domain.tld:8080/service:latest", "subdomain.domain.tld/service", "latest", "subdomain.domain.tld__service", "subdomain.domain.tld-service"},
	{"subdomain.domain.tld:8080/service:version", "subdomain.domain.tld:8080/service:version", "subdomain.domain.tld/service", "version", "subdomain.domain.tld__service", "subdomain.domain.tld-service"},
	{"subdomain.domain.tld:8080/namespace/service", "subdomain.domain.tld:8080/namespace/service:latest", "subdomain.domain.tld/namespace/service", "latest", "subdomain.domain.tld__namespace__service", "subdomain.domain.tld-namespace-service"},
	{"subdomain.domain.tld:8080/namespace/service:version", "subdomain.domain.tld:8080/namespace/service:version", "subdomain.domain.tld/namespace/service", "version", "subdomain.domain.tld__namespace__service", "subdomain.domain.tld-namespace-service"},
	{"service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "service", ""},
	{"namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "namespace/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "namespace__service", "namespace-service"},
	{"domain.tld/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld__service", "domain.tld-service"},
	{"domain.tld/namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/namespace/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld:8080/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld:8080/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld__service", "domain.tld-service"},
	{"domain.tld:8080/namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld:8080/namespace/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld/namespace/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"subdomain.domain.tld:8080/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "subdomain.domain.tld:8080/service@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "subdomain.domain.tld/service", "@sha256:123456789012345678901234567890123456789012345678901234567890abcd", "subdomain.domain.tld__service", "subdomain.domain.tld-service"},
}
