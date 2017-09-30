package utils

import (
	"fmt"
	"os"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// GetAuthorizer gets an Azure Service Principal authorizer.
// This func assumes "AZURE_TENANT_ID", "AZURE_CLIENT_ID",
// "AZURE_CLIENT_SECRET" are set as environment variables.
func GetAuthorizer(env azure.Environment) (*autorest.BearerAuthorizer, error) {
	tenantID := GetEnvVarOrExit("AZURE_TENANT_ID")

	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantID)
	if err != nil {
		return nil, err
	}

	clientID := GetEnvVarOrExit("AZURE_CLIENT_ID")
	clientSecret := GetEnvVarOrExit("AZURE_CLIENT_SECRET")

	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientID, clientSecret, env.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	return autorest.NewBearerAuthorizer(spToken), nil
}

// GetEnvVarOrExit returns the value of specified environment variable or terminates if it's not defined.
func GetEnvVarOrExit(varName string) string {
	value := os.Getenv(varName)
	if value == "" {
		fmt.Printf("Missing environment variable %s\n", varName)
		os.Exit(1)
	}
	return value
}
