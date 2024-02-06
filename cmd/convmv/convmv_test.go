// Package convmv provides the convmv command.
package convmv

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/encoder"
	"golang.org/x/text/unicode/norm"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Some times used in the tests
var (
	t1    = fstest.Time("2001-02-03T04:05:06.499999999Z")
	debug = ``
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestConvmv(t *testing.T) {
	type args struct {
		ConvertAlgo     fs.Enum[convertChoices]
		ConvertBackAlgo fs.Enum[convertChoices]
		Lossless        bool // whether the ConvertBackAlgo is always losslessly invertible
		ExtraOpt        ConvOpt
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "NFC", args: args{ConvertAlgo: ConvToNFC, ConvertBackAlgo: ConvToNFD, Lossless: false}},
		{name: "NFD", args: args{ConvertAlgo: ConvToNFD, ConvertBackAlgo: ConvToNFC, Lossless: false}},
		{name: "NFKC", args: args{ConvertAlgo: ConvToNFKC, ConvertBackAlgo: ConvToNFKD, Lossless: false}},
		{name: "NFKD", args: args{ConvertAlgo: ConvToNFKD, ConvertBackAlgo: ConvToNFKC, Lossless: false}},
		{name: "base64", args: args{ConvertAlgo: ConvBase64Encode, ConvertBackAlgo: ConvBase64Decode, Lossless: true}},
		{name: "replace", args: args{ConvertAlgo: ConvFindReplace, ConvertBackAlgo: ConvFindReplace, Lossless: true, ExtraOpt: ConvOpt{FindReplace: []string{"bread,banana", "pie,apple", "apple,pie", "banana,bread"}}}},
		{name: "prefix", args: args{ConvertAlgo: ConvPrefix, ConvertBackAlgo: ConvTrimPrefix, Lossless: true, ExtraOpt: ConvOpt{Prefix: "PREFIX"}}},
		{name: "suffix", args: args{ConvertAlgo: ConvSuffix, ConvertBackAlgo: ConvTrimSuffix, Lossless: true, ExtraOpt: ConvOpt{Suffix: "SUFFIX"}}},
		{name: "truncate", args: args{ConvertAlgo: ConvTruncate, ConvertBackAlgo: ConvTruncate, Lossless: false, ExtraOpt: ConvOpt{Max: 10}}},
		{name: "encoder", args: args{ConvertAlgo: ConvEncoder, ConvertBackAlgo: ConvDecoder, Lossless: true, ExtraOpt: ConvOpt{Enc: encoder.OS}}},
		{name: "ISO-8859-1", args: args{ConvertAlgo: ConvISO8859_1, ConvertBackAlgo: ConvISO8859_1, Lossless: false}},
		{name: "charmap", args: args{ConvertAlgo: ConvCharmap, ConvertBackAlgo: ConvCharmap, Lossless: false, ExtraOpt: ConvOpt{CmapFlag: 3}}},
		{name: "lowercase", args: args{ConvertAlgo: ConvLowercase, ConvertBackAlgo: ConvUppercase, Lossless: false}},
		{name: "ascii", args: args{ConvertAlgo: ConvASCII, ConvertBackAlgo: ConvASCII, Lossless: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fstest.NewRun(t)
			defer r.Finalise()

			items := makeTestFiles(t, r, "dir1")
			deleteDSStore(t, r)
			r.CheckRemoteListing(t, items, nil)

			Opt = tt.args.ExtraOpt
			Opt.ConvertAlgo = tt.args.ConvertAlgo
			err := Convmv(context.Background(), r.Fremote, "")
			assert.NoError(t, err)
			compareNames(t, r, items)

			convertedItems := convertItems(t, items)
			Opt.ConvertAlgo = tt.args.ConvertBackAlgo
			err = Convmv(context.Background(), r.Fremote, "")
			assert.NoError(t, err)
			compareNames(t, r, convertedItems)

			if tt.args.Lossless {
				deleteDSStore(t, r)
				r.CheckRemoteItems(t, items...)
			}
		})
	}
}

const alphabet = "ƀɀɠʀҠԀڀڠݠހ߀ကႠᄀᄠᅀᆀᇠሀሠበዠጠᎠᏀᐠᑀᑠᒀᒠᓀᓠᔀᔠᕀᕠᖀᖠᗀᗠᘀᘠᙀᚠᛀកᠠᡀᣀᦀ᧠ᨠᯀᰀᴀ⇠⋀⍀⍠⎀⎠⏀␀─┠╀╠▀■◀◠☀☠♀♠⚀⚠⛀⛠✀✠❀➀➠⠀⠠⡀⡠⢀⢠⣀⣠⤀⤠⥀⥠⦠⨠⩀⪀⪠⫠⬀⬠⭀ⰀⲀⲠⳀⴀⵀ⺠⻀㇀㐀㐠㑀㑠㒀㒠㓀㓠㔀㔠㕀㕠㖀㖠㗀㗠㘀㘠㙀㙠㚀㚠㛀㛠㜀㜠㝀㝠㞀㞠㟀㟠㠀㠠㡀㡠㢀㢠㣀㣠㤀㤠㥀㥠㦀㦠㧀㧠㨀㨠㩀㩠㪀㪠㫀㫠㬀㬠㭀㭠㮀㮠㯀㯠㰀㰠㱀㱠㲀㲠㳀㳠㴀㴠㵀㵠㶀㶠㷀㷠㸀㸠㹀㹠㺀㺠㻀㻠㼀㼠㽀㽠㾀㾠㿀㿠䀀䀠䁀䁠䂀䂠䃀䃠䄀䄠䅀䅠䆀䆠䇀䇠䈀䈠䉀䉠䊀䊠䋀䋠䌀䌠䍀䍠䎀䎠䏀䏠䐀䐠䑀䑠䒀䒠䓀䓠䔀䔠䕀䕠䖀䖠䗀䗠䘀䘠䙀䙠䚀䚠䛀䛠䜀䜠䝀䝠䞀䞠䟀䟠䠀䠠䡀䡠䢀䢠䣀䣠䤀䤠䥀䥠䦀䦠䧀䧠䨀䨠䩀䩠䪀䪠䫀䫠䬀䬠䭀䭠䮀䮠䯀䯠䰀䰠䱀䱠䲀䲠䳀䳠䴀䴠䵀䵠䶀䷀䷠一丠乀习亀亠什仠伀传佀你侀侠俀俠倀倠偀偠傀傠僀僠儀儠兀兠冀冠净几刀删剀剠劀加勀勠匀匠區占厀厠叀叠吀吠呀呠咀咠哀哠唀唠啀啠喀喠嗀嗠嘀嘠噀噠嚀嚠囀因圀圠址坠垀垠埀埠堀堠塀塠墀墠壀壠夀夠奀奠妀妠姀姠娀娠婀婠媀媠嫀嫠嬀嬠孀孠宀宠寀寠尀尠局屠岀岠峀峠崀崠嵀嵠嶀嶠巀巠帀帠幀幠庀庠廀廠开张彀彠往徠忀忠怀怠恀恠悀悠惀惠愀愠慀慠憀憠懀懠戀戠所扠技抠拀拠挀挠捀捠掀掠揀揠搀搠摀摠撀撠擀擠攀攠敀敠斀斠旀无昀映晀晠暀暠曀曠最朠杀杠枀枠柀柠栀栠桀桠梀梠检棠椀椠楀楠榀榠槀槠樀樠橀橠檀檠櫀櫠欀欠歀歠殀殠毀毠氀氠汀池沀沠泀泠洀洠浀浠涀涠淀淠渀渠湀湠満溠滀滠漀漠潀潠澀澠激濠瀀瀠灀灠炀炠烀烠焀焠煀煠熀熠燀燠爀爠牀牠犀犠狀狠猀猠獀獠玀玠珀珠琀琠瑀瑠璀璠瓀瓠甀甠畀畠疀疠痀痠瘀瘠癀癠皀皠盀盠眀眠着睠瞀瞠矀矠砀砠础硠碀碠磀磠礀礠祀祠禀禠秀秠稀稠穀穠窀窠竀章笀笠筀筠简箠節篠簀簠籀籠粀粠糀糠紀素絀絠綀綠緀締縀縠繀繠纀纠绀绠缀缠罀罠羀羠翀翠耀耠聀聠肀肠胀胠脀脠腀腠膀膠臀臠舀舠艀艠芀芠苀苠茀茠荀荠莀莠菀菠萀萠葀葠蒀蒠蓀蓠蔀蔠蕀蕠薀薠藀藠蘀蘠虀虠蚀蚠蛀蛠蜀蜠蝀蝠螀螠蟀蟠蠀蠠血衠袀袠裀裠褀褠襀襠覀覠觀觠言訠詀詠誀誠諀諠謀謠譀譠讀讠诀诠谀谠豀豠貀負賀賠贀贠赀赠趀趠跀跠踀踠蹀蹠躀躠軀軠輀輠轀轠辀辠迀迠退造遀遠邀邠郀郠鄀鄠酀酠醀醠釀釠鈀鈠鉀鉠銀銠鋀鋠錀錠鍀鍠鎀鎠鏀鏠鐀鐠鑀鑠钀钠铀铠销锠镀镠門閠闀闠阀阠陀陠隀隠雀雠需霠靀靠鞀鞠韀韠頀頠顀顠颀颠飀飠餀餠饀饠馀馠駀駠騀騠驀驠骀骠髀髠鬀鬠魀魠鮀鮠鯀鯠鰀鰠鱀鱠鲀鲠鳀鳠鴀鴠鵀鵠鶀鶠鷀鷠鸀鸠鹀鹠麀麠黀黠鼀鼠齀齠龀龠ꀀꀠꁀꁠꂀꂠꃀꃠꄀꄠꅀꅠꆀꆠꇀꇠꈀꈠꉀꉠꊀꊠꋀꋠꌀꌠꍀꍠꎀꎠꏀꏠꐀꐠꑀꑠ꒠ꔀꔠꕀꕠꖀꖠꗀꗠꙀꚠꛀ꜀꜠ꝀꞀꡀ測試_Русский___ě_áñ"

var extras = []string{"apple", "banana", "appleappleapplebanana", "splitbananasplit"}

func makeTestFiles(t *testing.T, r *fstest.Run, dir string) []fstest.Item {
	t.Helper()
	n := 0
	// Create test files
	items := []fstest.Item{}
	for _, c := range alphabet {
		var out strings.Builder
		for i := rune(0); i < 32; i++ {
			out.WriteRune(c + i)
		}
		fileName := filepath.Join(dir, fmt.Sprintf("%04d-%s.txt", n, out.String()))
		fileName = strings.ToValidUTF8(fileName, "")

		if debug != "" {
			fileName = debug
		}

		item := r.WriteObject(context.Background(), fileName, fileName, t1)
		items = append(items, item)
		n++

		if debug != "" {
			break
		}
	}

	for _, extra := range extras {
		item := r.WriteObject(context.Background(), extra, extra, t1)
		items = append(items, item)
	}

	return items
}

func deleteDSStore(t *testing.T, r *fstest.Run) {
	ctxDSStore, fi := filter.AddConfig(context.Background())
	err := fi.AddRule(`+ *.DS_Store`)
	assert.NoError(t, err)
	err = fi.AddRule(`- **`)
	assert.NoError(t, err)
	err = operations.Delete(ctxDSStore, r.Fremote)
	assert.NoError(t, err)
}

func compareNames(t *testing.T, r *fstest.Run, items []fstest.Item) {
	var entries fs.DirEntries

	deleteDSStore(t, r)
	err := walk.ListR(context.Background(), r.Fremote, "", true, -1, walk.ListObjects, func(e fs.DirEntries) error {
		entries = append(entries, e...)
		return nil
	})
	assert.NoError(t, err)
	entries = slices.DeleteFunc(entries, func(E fs.DirEntry) bool { // remove those pesky .DS_Store files
		if strings.Contains(E.Remote(), ".DS_Store") {
			err := operations.DeleteFile(context.Background(), E.(fs.Object))
			assert.NoError(t, err)
			return true
		}
		return false
	})
	require.Equal(t, len(items), entries.Len())

	// sort by CONVERTED name
	slices.SortStableFunc(items, func(a, b fstest.Item) int {
		aConv, err := ConvertPath(a.Path, Opt.ConvertAlgo, false)
		require.NoError(t, err, a.Path)
		bConv, err := ConvertPath(b.Path, Opt.ConvertAlgo, false)
		require.NoError(t, err, b.Path)
		return cmp.Compare(aConv, bConv)
	})
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Remote(), b.Remote())
	})

	for i, e := range entries {
		expect, err := ConvertPath(items[i].Path, Opt.ConvertAlgo, false)
		assert.NoError(t, err)
		msg := fmt.Sprintf("expected %v, got %v", detectEncoding(expect), detectEncoding(e.Remote()))
		assert.Equal(t, expect, e.Remote(), msg)
	}
}

func convertItems(t *testing.T, items []fstest.Item) []fstest.Item {
	convertedItems := []fstest.Item{}
	for _, item := range items {
		newPath, err := ConvertPath(item.Path, Opt.ConvertAlgo, false)
		assert.NoError(t, err)
		newItem := item
		newItem.Path = newPath
		convertedItems = append(convertedItems, newItem)
	}
	return convertedItems
}

func detectEncoding(s string) string {
	if norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "BOTH"
	}
	if !norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "NFD"
	}
	if norm.NFC.IsNormalString(s) && !norm.NFD.IsNormalString(s) {
		return "NFC"
	}
	return "OTHER"
}
