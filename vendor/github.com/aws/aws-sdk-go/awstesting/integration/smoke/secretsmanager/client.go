// +build integration

//Package secretsmanager provides gucumber integration tests support.
package secretsmanager

import (
	"github.com/aws/aws-sdk-go/awstesting/integration/smoke"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/gucumber/gucumber"
)

func init() {
	gucumber.Before("@secretsmanager", func() {
		gucumber.World["client"] = secretsmanager.New(smoke.Session)
	})
}
