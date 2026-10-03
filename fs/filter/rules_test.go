package filter

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesDirectoryIndexMatchesLinear(t *testing.T) {
	patterns := []string{
		`^alpha/.*$`, `^beta/private/.*$`, `^beta/.*$`, `.*secret.*`,
		`^gamma/(?:one|two)/.*$`, `^a\.b/.*$`, `^日本語/.*$`,
		`(?i)^ALPHA/.*$`, `^alpha/.*$|beta/.*$`, `(?m)^alpha/.*$`,
		`^(alpha/.*)$`, `^replacement�/.*$`, `^alpha/file$`, `.*`,
	}
	remotes := []string{
		"", "alpha", "alpha/", "alpha/file", "alpha/secret", "ALPHA/file",
		"beta/private/file", "beta/public/file", "gamma/one/file", "gamma/three/file",
		"a.b/file", "axb/file", "日本語/file", "unknown/file", "prefix\nalpha/file",
		"replacement�/file", "replacement\xff/file", "replacement\xff\xff/file",
	}
	// Move generic rules through the list to check first-match precedence.
	for _, count := range []int{1, 63, 64, 65, 128} {
		for offset := range patterns {
			t.Run(fmt.Sprintf("rules=%d/offset=%d", count, offset), func(t *testing.T) {
				var rs rules
				for i := range count {
					rs.add(i%2 == 0, regexp.MustCompile(fmt.Sprintf(`^padding_%d/.*$`, i)))
				}
				for i := range patterns {
					rs.add(i%2 == 0, regexp.MustCompile(patterns[(i+offset)%len(patterns)]))
				}
				for _, remote := range remotes {
					want := true
					for _, r := range rs.rules {
						if r.Match(remote) {
							want = r.Include
							break
						}
					}
					assert.Equal(t, want, rs.include(remote), "remote %q", remote)
				}
			})
		}
	}
}

func TestRulesDirectoryIndexAddAndClear(t *testing.T) {
	var rs rules
	for i := range 128 {
		require.NoError(t, rs.Add(false, fmt.Sprintf("/padding_%d/**", i)))
	}
	require.NoError(t, rs.Add(false, "/target/private/**"))
	require.NoError(t, rs.Add(true, "/target/**"))
	count := rs.len()
	require.NoError(t, rs.Add(true, "/target/**"))
	assert.Equal(t, count, rs.len())
	require.NoError(t, rs.Add(false, "**"))
	assert.False(t, rs.include("target/private/file"))
	assert.True(t, rs.include("target/public/file"))
	assert.False(t, rs.include("other/file"))
	rs.clear()
	assert.Zero(t, rs.len())
	assert.True(t, rs.include("other/file"))
	require.NoError(t, rs.Add(false, "/target/**"))
	assert.False(t, rs.include("target/public/file"))
	assert.True(t, rs.include("other/file"))
	// Rebuild an index after a reset, with the opposite rule for the same path.
	for i := range 128 {
		require.NoError(t, rs.Add(false, fmt.Sprintf("/padding_%d/**", i)))
	}
	assert.False(t, rs.include("target/public/file"))
}

func BenchmarkFilterLiteralDirectories(b *testing.B) {
	for _, count := range []int{4, 32, 1000, 10000, 100000} {
		b.Run(fmt.Sprintf("rules=%d", count), func(b *testing.B) {
			opt := Opt
			opt.IncludeRule = make([]string, count)
			for i := range opt.IncludeRule {
				opt.IncludeRule[i] = fmt.Sprintf("/dataset_%06d/**", i)
			}
			f, err := NewFilter(&opt)
			if err != nil {
				b.Fatal(err)
			}
			include := f.IncludeDirectory(context.Background(), nil)
			for _, remote := range []string{"dataset_000000", fmt.Sprintf("dataset_%06d", count-1), "unselected_directory"} {
				b.Run(remote, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						_, err := include(remote)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func BenchmarkFilterLiteralDirectoriesSetup(b *testing.B) {
	for _, count := range []int{32, 1000, 10000} {
		b.Run(fmt.Sprintf("rules=%d", count), func(b *testing.B) {
			opt := Opt
			opt.IncludeRule = make([]string, count)
			for i := range opt.IncludeRule {
				opt.IncludeRule[i] = fmt.Sprintf("/dataset_%06d/**", i)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := NewFilter(&opt); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
