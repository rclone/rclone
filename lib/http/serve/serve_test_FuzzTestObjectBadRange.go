package serve

import (
	"net/http/httptest"
	"os"
	"secsys/gout-transformation/pkg/transstruct"
	"testing"

	"github.com/rclone/rclone/fstest/mockobject"
)

func FuzzTestObjectBadRange(XVl []byte) int {
	t := &testing.T{}
	_ = t
	var skippingTableDriven bool
	_, skippingTableDriven = os.LookupEnv("SKIPPING_TABLE_DRIVEN")
	_ = skippingTableDriven
	transstruct.SetFuzzData(XVl)
	FDG_FuzzGlobal()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/aFile", nil)
	r.Header.Add("Range", "xxxbytes=3-5")
	o := mockobject.New(transstruct.GetString("aFile")).WithContent([]byte(transstruct.GetString("0123456789")), mockobject.SeekModeNone)
	Object(w, r, o)
	_ = w.Result()

	return 1
}

func FDG_FuzzGlobal() {

}
