package rdsutils_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"

	"github.com/stretchr/testify/assert"
)

func TestBuildAuthToken(t *testing.T) {
	endpoint := "https://prod-instance.us-east-1.rds.amazonaws.com:3306"
	region := "us-west-2"
	creds := credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")
	user := "mysqlUser"

	url, err := rdsutils.BuildAuthToken(endpoint, region, user, creds)
	assert.NoError(t, err)
	assert.Regexp(t, `^prod-instance\.us-east-1\.rds\.amazonaws\.com:3306\?Action=connect.*?DBUser=mysqlUser.*`, url)
}
