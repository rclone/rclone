package proton_test

import (
	"testing"

	"github.com/rclone/go-proton-api"
)

func Test_HMAC(t *testing.T) {
	hashKey := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	check := func(t *testing.T, str, ans string) {
		ret, err := proton.GetNameHash(str, hashKey)
		if err != nil {
			t.Fatal(err)
		}
		if ret != ans {
			t.Fatal("Mismatching HMAC", ans, ret)
		}
	}

	check(t, "garçon", "02ef4861a4b9f833aa104a8210f5eb338e231c9532d9c2551aaf76bafb511208")
	check(t, "apă", "fd80de16c11bdcea2783274f6b7f334093ef95d58c7381c615005614ed77dc94")
	check(t, "bala", "35733f41071d4997876b5bb54acc1d587646bdf1251f9b9c49ee9dc023a69962")
	check(t, "țânțar", "4f4dee0cd87928027982c6ca280d2c7661073082ed46a82c111880126b0c3e14")
	check(t, "întuneric", "6bed2bff136e165ad54d0a2a9a549481c88aca55d567c93123e0e5b876c291b2")
	check(t, "mädchen", "2b112b1b7ac4fd9dae5a2acd8fcf2e905bd92a06a95dc4495fa012bda93e8607")
	check(t, "integrationTestImage.png", "2e700ef3b52379a9277ac48bcfc5dff56e6927274267a0df4673f4f21e5d04d6")
}
