package fs

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMountHelperArgs(t *testing.T) {
	type testCase struct {
		src []string
		dst []string
		env string
		err string
	}
	normalCases := []testCase{{
		src: []string{},
		dst: []string{"mount", "--daemon"},
	}, {
		src: []string{"-o", `x-systemd.automount,vvv,env.HTTPS_PROXY="a b;c,d?EF",ro,rw,args2env,_netdev`},
		dst: []string{"mount", "--read-only", "--verbose=3", "--daemon"},
		env: "HTTPS_PROXY=a b;c,d?EF",
	}}

	for _, tc := range normalCases {
		exe := []string{"rclone"}
		src := append(exe, tc.src...)
		res, err := convertMountHelperArgs(src)

		if tc.err != "" {
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
			continue
		}

		require.NoError(t, err)
		require.Greater(t, len(res), 1)
		assert.Equal(t, exe[0], res[0])
		dst := res[1:]

		//log.Printf("%q -> %q", tc.src, dst)
		assert.Equal(t, tc.dst, dst)

		if tc.env != "" {
			idx := strings.Index(tc.env, "=")
			name, value := tc.env[:idx], tc.env[idx+1:]
			assert.Equal(t, value, os.Getenv(name))
		}
	}
}
