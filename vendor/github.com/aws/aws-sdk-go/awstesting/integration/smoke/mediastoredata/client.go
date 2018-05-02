// +build integration

//Package mediastoredata provides gucumber integration tests support.
package mediastoredata

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/awstesting/integration/smoke"
	"github.com/aws/aws-sdk-go/service/mediastore"
	"github.com/aws/aws-sdk-go/service/mediastoredata"
	"github.com/gucumber/gucumber"
)

func init() {
	const containerName = "awsgosdkteamintegcontainer"

	gucumber.Before("@mediastoredata", func() {
		mediastoreSvc := mediastore.New(smoke.Session)

		resp, err := mediastoreSvc.DescribeContainer(&mediastore.DescribeContainerInput{
			ContainerName: aws.String(containerName),
		})
		if err != nil {
			gucumber.World["error"] = fmt.Errorf("failed to get mediastore container endpoint for test, %v", err)
			return
		}

		gucumber.World["client"] = mediastoredata.New(smoke.Session, &aws.Config{
			Endpoint: resp.Container.Endpoint,
		})
	})
}
