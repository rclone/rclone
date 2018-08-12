package api

import (
	"encoding/xml"
	"testing"
	"time"
)

func TestMountpointEmptyModificationTime(t *testing.T) {
	mountpoint := `
<mountPoint time="2018-08-12-T09:58:24Z" host="dn-157">
  <name xml:space="preserve">Sync</name>
  <path xml:space="preserve">/foo/Jotta</path>
  <abspath xml:space="preserve">/foo/Jotta</abspath>
  <size>0</size>
  <modified></modified>
  <device>Jotta</device>
  <user>foo</user>
  <metadata first="" max="" total="0" num_folders="0" num_files="0"/>
</mountPoint>
`
	var jf JottaFolder
	if err := xml.Unmarshal([]byte(mountpoint), &jf); err != nil {
		t.Fatal(err)
	}
	if !time.Time(jf.ModifiedAt).IsZero() {
		t.Errorf("got non-zero time, want zero")
	}
}
