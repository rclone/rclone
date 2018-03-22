// +build integration

//Package sagemakerruntime provides gucumber integration tests support.
package sagemakerruntime

import (
	"github.com/aws/aws-sdk-go/awstesting/integration/smoke"
	"github.com/aws/aws-sdk-go/service/sagemakerruntime"
	"github.com/gucumber/gucumber"
)

func init() {
	gucumber.Before("@sagemakerruntime", func() {
		gucumber.World["client"] = sagemakerruntime.New(smoke.Session)
	})
}
