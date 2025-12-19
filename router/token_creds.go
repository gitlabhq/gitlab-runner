package router

import (
	"context"

	"google.golang.org/grpc/credentials"
)

const (
	metadataAuthorization = "authorization"
	metadataAgentType     = "gitlab-agent-type"
)

func newTokenCredentials(token string, insecure bool) credentials.PerRPCCredentials {
	return &tokenCredentials{
		metadata: map[string]string{
			metadataAuthorization: "Bearer " + token,
			metadataAgentType:     "runner",
		},
		insecure: insecure,
	}
}

type tokenCredentials struct {
	metadata map[string]string
	insecure bool
}

func (t *tokenCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return t.metadata, nil
}

func (t *tokenCredentials) RequireTransportSecurity() bool {
	return !t.insecure
}
