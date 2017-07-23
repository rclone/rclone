package rdsutils_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"

	"github.com/stretchr/testify/assert"
)

func TestBuildAuthToken(t *testing.T) {
	cases := []struct {
		endpoint      string
		region        string
		user          string
		expectedRegex string
	}{
		{
			"https://prod-instance.us-east-1.rds.amazonaws.com:3306",
			"us-west-2",
			"mysqlUser",
			`^prod-instance\.us-east-1\.rds\.amazonaws\.com:3306\?Action=connect.*?DBUser=mysqlUser.*`,
		},
		{
			"prod-instance.us-east-1.rds.amazonaws.com:3306",
			"us-west-2",
			"mysqlUser",
			`^prod-instance\.us-east-1\.rds\.amazonaws\.com:3306\?Action=connect.*?DBUser=mysqlUser.*`,
		},
	}

	for _, c := range cases {
		creds := credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")
		url, err := rdsutils.BuildAuthToken(c.endpoint, c.region, c.user, creds)
		assert.NoError(t, err)
		assert.Regexp(t, c.expectedRegex, url)
	}
}
