package filename

import (
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
		wantErr bool
	}{
		{
			name: "uncompressed",
			// tableUncompressed
			encoded: "AYS5i",
			want:    "a.b",
		},
		{
			name: "uncompressed-long",
			// tableUncompressed
			encoded: "AQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZQnpHUVl4cUhCQTZsalRzaXI4MGdVTTVZ",
			want:    "BzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5YBzGQYxqHBA6ljTsir80gUM5Y",
		},
		{
			name: "plain-1",
			// Table 2
			encoded: "BzGQYxqHBA6ljTsir80gUM5Y=",
			want:    "-Duplican99E8ZI4___9_",
		},
		{
			name: "hex-1",
			// Table 4
			encoded: "D_--tHZROQpqqJ9PafqNa6STF",
			want:    "13646871dfabbs43323564654bbefff",
		},
		{
			name: "hex-2",
			// Table 6
			encoded: "GhIEAIOBQMFQeWm4SClVpXVldCXFZLj4uOgoJHChQ4KBiXQ==",
			want:    "5368616e6e6f6e206c696d69743a203534353833206279746573-+._=!()",
		},
		{
			name: "hex-3",
			// Table 7
			encoded: "HohwXBXoJcVFSHgpdVQlxHXIuVgpNCR06Eg5aBg==",
			want:    "7461626C6520312073697A653A203335206572723A203C6E696C3E",
		},
		{
			name: "base64-1",
			// Table 5
			encoded: "FMpABB9Ef0KP8OrVxjnE3LzUePuLZi8pPg7eW8bgyW2d3Ucckf4rlE0mkAvlILVpOmF3L-rFbmNrpUO2HQFlF4SCMPVPeCEX6LeOg5JVpUVCXV1WSazD9vSpr",
			want:    "UxAYiB0FNTTkXRw9P8hwq-WmN7tYwbe-sFw8C3snDRG1d-yjrdOUVZQyLdtkJ8tuvhBSnuBiLjVieCAroWEZDIO4Hb_rKgdzPjMqFE7inwHJ2isF==",
		},
		{
			name: "custom-1",
			// Table 62, custom
			encoded: "-BeADJCoG_________________xc=",
			want:    "Uaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name: "custom-2",
			// Table 62, custom
			encoded: "-BPABDWUppYyllDKW0sYYSymljJQx",
			want:    "12312132123121321321321321312312312313132132131231213213213213123121321321321",
		},
		{
			name: "rle-1",
			// tableRLE
			encoded: "9a2E=",
			want:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name: "regular-1",
			// Table 1
			encoded: "BeSSrnzj0j3OXyR9K81M=",
			want:    "regular-filename.txt",
		},
		{
			name: "regular-3",
			// Table 2
			encoded: "COyCCD-42d9s=",
			want:    "00012312.JPG",
		},
		{
			name: "regular-4",
			// Table 3
			encoded: "DmqiJmrhNSDOJTCKTyCQ=",
			want:    ". . . .txta123123123",
		},
		{
			name: "unicode-1",
			// tableSCSUPlain
			encoded: "8D5V3MESVd-WEF7WuqaOvpKUWtYGEyw5UDQ==",
			want:    "長い長いＵＮＩＣＯＤＥファイル名",
		},
		{
			name: "unicode-2",
			// tableSCSUPlain
			encoded: "8GyHV1N7u2OEg4ufQ3eHQ3Ngg6N3X0CDg4-HX0NXU2tg=",
			want:    "ვეპხის ტყაოსანი შოთა რუსთაველი",
		},
		{
			name: "unicode-3",
			// tableSCSU
			encoded: "7LpehMXOrWe7mcT_lpf2MN1Nmgu55jpXHLavZcXJb2UTJ-UmGU15iznkD",
			want:    "Sønderjysk: Æ ka æe glass uhen at det go mæ naue.,",
		},
		{
			name: "unicode-4",
			// tableSCSU
			encoded: "7TCSRm0liJDR0ulpBq4Lla_XB2mWdLFMEs8wEQKHAGa8FRr333ntJ6Ww6_f__N5VKeYM=",
			want:    "Hello------world     時危兵甲滿天涯，載道流離起怨咨.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.encoded)
			if (err != nil) != tt.wantErr {
				if tt.encoded == "" && tt.want != "" {
					proposed := Encode(tt.want)
					table := decodeMap[proposed[0]] - 1
					t.Errorf("No encoded value, try '%s', table is %d", proposed, table)
					return
				}
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				proposed := Encode(tt.want)
				table := decodeMap[proposed[0]] - 1
				if len(proposed) > len(tt.encoded) {
					t.Errorf("Got longer encoded value than reference. Likely compression regression. Got %s, table %d", proposed, table)
				}
				if len(proposed) > len(tt.encoded) {
					t.Logf("Got better encoded value, improved length %d, was %d", len(proposed), len(tt.encoded))
				}
			}

			if got != tt.want {
				t.Errorf("Decode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
