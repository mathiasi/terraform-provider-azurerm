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

type servicePrincipalClientSecretMultiTenantAuth struct {
	ctx                context.Context
	clientId           string
	clientSecret       string
	environment        string
	subscriptionId     string
	tenantId           string
	tenantOnly         bool
	auxiliaryTenantIDs []string
}

func (a servicePrincipalClientSecretMultiTenantAuth) build(b Builder) (authMethod, error) {
	method := servicePrincipalClientSecretMultiTenantAuth{
		ctx:                b.Context,
		clientId:           b.ClientID,
		clientSecret:       b.ClientSecret,
		environment:        b.Environment,
		subscriptionId:     b.SubscriptionID,
		tenantId:           b.TenantID,
		tenantOnly:         b.TenantOnly,
		auxiliaryTenantIDs: b.AuxiliaryTenantIDs,
	}
	return method, nil
}

func (a servicePrincipalClientSecretMultiTenantAuth) isApplicable(b Builder) bool {
	return b.SupportsClientSecretAuth && b.ClientSecret != "" && b.SupportsAuxiliaryTenants && (len(b.AuxiliaryTenantIDs) > 0)
}

func (a servicePrincipalClientSecretMultiTenantAuth) name() string {
	return "Multi Tenant Service Principal / Client Secret"
}

func (a servicePrincipalClientSecretMultiTenantAuth) getAuthorizationToken(sender autorest.Sender, oauth *OAuthConfig, endpoint string) (autorest.Authorizer, error) {
	if oauth.MultiTenantOauth == nil {
		return nil, fmt.Errorf("getting Authorization Token for client cert: an MultiTenantOauth token wasn't configured correctly; please file a bug with more details")
	}

	spt, err := adal.NewMultiTenantServicePrincipalToken(*oauth.MultiTenantOauth, a.clientId, a.clientSecret, endpoint)
	if err != nil {
		return nil, err
	}

	spt.PrimaryToken.SetSender(sender)
	for _, t := range spt.AuxiliaryTokens {
		t.SetSender(sender)
	}

	auth := autorest.NewMultiTenantServicePrincipalTokenAuthorizer(spt)
	return auth, nil
}

func (a servicePrincipalClientSecretMultiTenantAuth) getAuthorizationTokenV2(_ autorest.Sender, _ *OAuthConfig, endpoint string) (autorest.Authorizer, error) {
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

func (a servicePrincipalClientSecretMultiTenantAuth) populateConfig(c *Config) error {
	c.AuthenticatedAsAServicePrincipal = true
	c.GetAuthenticatedObjectID = buildServicePrincipalObjectIDFunc(c)
	return nil
}

func (a servicePrincipalClientSecretMultiTenantAuth) validate() error {
	var err *multierror.Error

	fmtErrorMessage := "%s must be configured when authenticating as a Service Principal using a Multi Tenant Client Secret."

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
	if len(a.auxiliaryTenantIDs) == 0 {
		err = multierror.Append(err, fmt.Errorf(fmtErrorMessage, "Auxiliary Tenant IDs"))
	}
	return err.ErrorOrNil()
}
