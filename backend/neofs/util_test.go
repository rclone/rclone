package neofs

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEndpoints(t *testing.T) {
	for i, tc := range []struct {
		EndpointsParam string
		ExpectedError  bool
		ExpectedResult []EndpointInfo
	}{
		{
			EndpointsParam: "s01.neofs.devenv:8080",
			ExpectedResult: []EndpointInfo{{
				Address:  "s01.neofs.devenv:8080",
				Priority: 1,
				Weight:   1,
			}},
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,2",
			ExpectedResult: []EndpointInfo{{
				Address:  "s01.neofs.devenv:8080",
				Priority: 2,
				Weight:   1,
			}},
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,2,3",
			ExpectedResult: []EndpointInfo{{
				Address:  "s01.neofs.devenv:8080",
				Priority: 2,
				Weight:   3,
			}},
		},
		{
			EndpointsParam: " s01.neofs.devenv:8080  s02.neofs.devenv:8080 ",
			ExpectedResult: []EndpointInfo{
				{
					Address:  "s01.neofs.devenv:8080",
					Priority: 1,
					Weight:   1,
				},
				{
					Address:  "s02.neofs.devenv:8080",
					Priority: 1,
					Weight:   1,
				},
			},
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,1,1 s02.neofs.devenv:8080,2,1 s03.neofs.devenv:8080,2,9",
			ExpectedResult: []EndpointInfo{
				{
					Address:  "s01.neofs.devenv:8080",
					Priority: 1,
					Weight:   1,
				},
				{
					Address:  "s02.neofs.devenv:8080",
					Priority: 2,
					Weight:   1,
				},
				{
					Address:  "s03.neofs.devenv:8080",
					Priority: 2,
					Weight:   9,
				},
			},
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,-1,1",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,,",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,sd,sd",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,1,0",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,1 s02.neofs.devenv:8080",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,1,2 s02.neofs.devenv:8080",
			ExpectedError:  true,
		},
		{
			EndpointsParam: "s01.neofs.devenv:8080,1,2 s02.neofs.devenv:8080,1",
			ExpectedError:  true,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			res, err := parseEndpoints(tc.EndpointsParam)
			if tc.ExpectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.ExpectedResult, res)
		})
	}
}
