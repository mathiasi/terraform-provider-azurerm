package authentication

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/hashicorp/go-multierror"
	"github.com/manicminer/hamilton/auth"
	"github.com/manicminer/hamilton/environments"
)

type servicePrincipalClientSecretAuth struct {
	ctx            context.Context
	clientId       string
	clientSecret   string
	environment    string
	subscriptionId string
	tenantId       string
	tenantOnly     bool
}

func (a servicePrincipalClientSecretAuth) build(b Builder) (authMethod, error) {
	method := servicePrincipalClientSecretAuth{
		ctx:            b.Context,
		clientId:       b.ClientID,
		clientSecret:   b.ClientSecret,
		environment:    b.Environment,
		subscriptionId: b.SubscriptionID,
		tenantId:       b.TenantID,
		tenantOnly:     b.TenantOnly,
	}
	return method, nil
}

func (a servicePrincipalClientSecretAuth) isApplicable(b Builder) bool {
	return b.SupportsClientSecretAuth && b.ClientSecret != ""
}

func (a servicePrincipalClientSecretAuth) name() string {
	return "Service Principal / Client Secret"
}

func (a servicePrincipalClientSecretAuth) getAuthorizationToken(sender autorest.Sender, oauth *OAuthConfig, endpoint string) (autorest.Authorizer, error) {
	if oauth.OAuth == nil {
		return nil, fmt.Errorf("getting Authorization Token for client secret auth: an OAuth token wasn't configured correctly; please file a bug with more details")
	}

	spt, err := adal.NewServicePrincipalToken(*oauth.OAuth, a.clientId, a.clientSecret, endpoint)
	if err != nil {
		return nil, err
	}
	spt.SetSender(sender)

	return autorest.NewBearerAuthorizer(spt), nil
}

func (a servicePrincipalClientSecretAuth) getAuthorizationTokenV2(_ autorest.Sender, _ *OAuthConfig, endpoint string) (autorest.Authorizer, error) {
	environment, err := environments.EnvironmentFromString(a.environment)
	if err != nil {
		return nil, fmt.Errorf("environment config error: %v", err)
	}

	conf := auth.ClientCredentialsConfig{
		Environment:  environment,
		TenantID:     a.tenantId,
		ClientID:     a.clientId,
		ClientSecret: a.clientSecret,
		Scopes:       []string{fmt.Sprintf("%s/.default", strings.TrimRight(endpoint, "/"))},
		TokenVersion: auth.TokenVersion2,
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	authorizer := conf.TokenSource(ctx, auth.ClientCredentialsSecretType)
	if authTyped, ok := authorizer.(autorest.Authorizer); ok {
		return authTyped, nil
	}

	return nil, fmt.Errorf("returned auth.Authorizer does not implement autorest.Authorizer")
}

func (a servicePrincipalClientSecretAuth) populateConfig(c *Config) error {
	c.AuthenticatedAsAServicePrincipal = true
	c.GetAuthenticatedObjectID = buildServicePrincipalObjectIDFunc(c)
	return nil
}

func (a servicePrincipalClientSecretAuth) validate() error {
	var err *multierror.Error

	fmtErrorMessage := "A %s must be configured when authenticating as a Service Principal using a Client Secret."

	if !a.tenantOnly && a.subscriptionId == "" {
		err = multierror.Append(err, fmt.Errorf(fmtErrorMessage, "Subscription ID"))
	}
	if a.clientId == "" {
		err = multierror.Append(err, fmt.Errorf(fmtErrorMessage, "Client ID"))
	}
	if a.clientSecret == "" {
		err = multierror.Append(err, fmt.Errorf(fmtErrorMessage, "Client Secret"))
	}
	if a.tenantId == "" {
		err = multierror.Append(err, fmt.Errorf(fmtErrorMessage, "Tenant ID"))
	}

	return err.ErrorOrNil()
}
