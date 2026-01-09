package llmapi

import (
	"context"

	v1 "github.com/xucx/llmapi/api/v1"
	"github.com/xucx/llmapi/internal/providers/provider"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ProviderName = "llmapi"
)

var (
	_ provider.Provider = (*LLMApiProvider)(nil)
)

type LLMApiProvider struct {
	provider.ProviderNop
	apiClient v1.ApiServiceClient
}

func NewLLMApiProvider(opts ...provider.ProviderOption) (provider.Provider, error) {
	options := provider.GetProviderOptions(opts...)

	dailOptions := []grpc.DialOption{
		grpc.WithPerRPCCredentials(&tokenAuth{
			token: options.Sk,
		}),
	}

	if options.Insecure {
		dailOptions = append(dailOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	client, err := grpc.NewClient(options.Url, dailOptions...)
	if err != nil {
		return nil, err
	}

	apiClient := v1.NewApiServiceClient(client)

	return &LLMApiProvider{apiClient: apiClient}, nil
}

type tokenAuth struct {
	token string
}

func (t *tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"Authorization": "Bearer " + t.token,
	}, nil
}

func (t *tokenAuth) RequireTransportSecurity() bool {
	return false
}
