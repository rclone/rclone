package filename

import "testing"

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
		wantErr bool
	}{
		{
			name:    "unicode-1",
			encoded: "8D5V3MESVd-WEF7WuqaOvpKUWtYGEyw5UDQ==",
			want:    "長い長いＵＮＩＣＯＤＥファイル名",
			wantErr: false,
		},
		{
			name:    "unicode-2",
			encoded: "8GyHV1N7u2OEg4ufQ3eHQ3Ngg6N3X0CDg4-HX0NXU2tg=",
			want:    "ვეპხის ტყაოსანი შოთა რუსთაველი",
			wantErr: false,
		},
		{
			name:    "unicode-3",
			encoded: "7LpehMXOrWe7mcT_lpf2MN1Nmgu55jpXHLavZcXJb2UTJ-UmGU15iznkD",
			want:    "Sønderjysk: Æ ka æe glass uhen at det go mæ naue.,",
			wantErr: false,
		},
		{
			name:    "unicode-4",
			encoded: "7TCSRm0liJDR0ulpBq4Lla_XB2mWdLFMEs8wEQKHAGa8FRr333ntJ6Ww6_f__N5VKeYM=",
			want:    "Hello------world     時危兵甲滿天涯，載道流離起怨咨.bin",
		},
		{
			name:    "plain-1",
			encoded: "BzGQYxqHBA6ljTsir80gUM5Y=",
			want:    "-Duplican99E8ZI4___9_",
			wantErr: false,
		},
		{
			name:    "hex-1",
			encoded: "D_--tHZROQpqqJ9PafqNa6STF",
			want:    "13646871dfabbs43323564654bbefff",
			wantErr: false,
		},
		{
			name:    "base64-1",
			encoded: "FMpABB9Ef0KP8OrVxjnE3LzUePuLZi8pPg7eW8bgyW2d3Ucckf4rlE0mkAvlILVpOmF3L-rFbmNrpUO2HQFlF4SCMPVPeCEX6LeOg5JVpUVCXV1WSazD9vSpr",
			want:    "UxAYiB0FNTTkXRw9P8hwq-WmN7tYwbe-sFw8C3snDRG1d-yjrdOUVZQyLdtkJ8tuvhBSnuBiLjVieCAroWEZDIO4Hb_rKgdzPjMqFE7inwHJ2isF==",
			wantErr: false,
		},
		{
			name:    "custom-1",
			encoded: "-BeADJCoG_________________xc=",
			want:    "Uaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantErr: false,
		},
		{
			name:    "rle-1",
			encoded: "9a2E=",
			want:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantErr: false,
		},
		{
			name:    "regular-1",
			encoded: "BeSSrnzj0j3OXyR9K81M=",
			want:    "regular-filename.txt",
			wantErr: false,
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

			if got != tt.want {
				t.Errorf("Decode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
