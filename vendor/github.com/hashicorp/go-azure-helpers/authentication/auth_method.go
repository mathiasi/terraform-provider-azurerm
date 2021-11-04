package authentication

import (
	"context"

	"github.com/Azure/go-autorest/autorest"
	"github.com/manicminer/hamilton/environments"
)

type authMethod interface {
	build(b Builder) (authMethod, error)

	isApplicable(b Builder) bool

	getAuthorizationToken(sender autorest.Sender, oauthConfig *OAuthConfig, endpoint string) (autorest.Authorizer, error)

	getAuthorizationTokenV2(ctx context.Context, environment environments.Environment, tenantId string, scopes []string) (autorest.Authorizer, error)

	name() string

	populateConfig(c *Config) error

	validate() error
}
