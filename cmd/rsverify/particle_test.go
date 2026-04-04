package main

import (
	"testing"
	"time"

	"github.com/rclone/rclone/backend/rs"
)

func TestTryParticleFooter(t *testing.T) {
	payload := []byte("hello-world-payload")
	ft := rs.NewRSFooter(42, nil, nil, time.Unix(1700000000, 0), 4, 3, 2, 9, rs.CRC32C(payload))
	fb, err := ft.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	raw := append(append([]byte{}, payload...), fb...)
	gotPayload, gotFt, ok := tryParticleFooter(raw)
	if !ok {
		t.Fatal("expected valid particle")
	}
	if string(gotPayload) != string(payload) {
		t.Fatalf("payload: got %q", gotPayload)
	}
	if gotFt.CurrentShard != 2 || gotFt.DataShards != 4 || gotFt.ParityShards != 3 {
		t.Fatalf("footer fields: %+v", gotFt)
	}
}
