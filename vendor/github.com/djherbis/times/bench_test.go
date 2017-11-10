package times

import (
	"os"
	"testing"
)

func BenchmarkGet(t *testing.B) {
	fileTest(t, func(f *os.File) {
		fi, err := os.Stat(f.Name())
		if err != nil {
			t.Error(err)
		}

		for i := 0; i < t.N; i++ {
			Get(fi)
		}
	})
	t.ReportAllocs()
}

func BenchmarkStat(t *testing.B) {
	fileTest(t, func(f *os.File) {
		for i := 0; i < t.N; i++ {
			Stat(f.Name())
		}
	})
	t.ReportAllocs()
}
