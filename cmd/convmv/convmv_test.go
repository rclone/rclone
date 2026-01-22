// Package convmv provides the convmv command.
package convmv

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/transform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
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

func TestTransform(t *testing.T) {
	type args struct {
		TransformOpt     []string
		TransformBackOpt []string
		Lossless         bool // whether the TransformBackAlgo is always losslessly invertible
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "NFC", args: args{
			TransformOpt:     []string{"all,nfc"},
			TransformBackOpt: []string{"all,nfd"},
			Lossless:         false,
		}},
		{name: "NFD", args: args{
			TransformOpt:     []string{"all,nfd"},
			TransformBackOpt: []string{"all,nfc"},
			Lossless:         false,
		}},
		{name: "base64", args: args{
			TransformOpt:     []string{"all,base64encode"},
			TransformBackOpt: []string{"all,base64encode"},
			Lossless:         false,
		}},
		{name: "prefix", args: args{
			TransformOpt:     []string{"all,prefix=PREFIX"},
			TransformBackOpt: []string{"all,trimprefix=PREFIX"},
			Lossless:         true,
		}},
		{name: "suffix", args: args{
			TransformOpt:     []string{"all,suffix=SUFFIX"},
			TransformBackOpt: []string{"all,trimsuffix=SUFFIX"},
			Lossless:         true,
		}},
		{name: "truncate", args: args{
			TransformOpt:     []string{"all,truncate=10"},
			TransformBackOpt: []string{"all,truncate=10"},
			Lossless:         false,
		}},
		{name: "encoder", args: args{
			TransformOpt:     []string{"all,encoder=Colon,SquareBracket"},
			TransformBackOpt: []string{"all,decoder=Colon,SquareBracket"},
			Lossless:         true,
		}},
		{name: "ISO-8859-1", args: args{
			TransformOpt:     []string{"all,ISO-8859-1"},
			TransformBackOpt: []string{"all,ISO-8859-1"},
			Lossless:         false,
		}},
		{name: "charmap", args: args{
			TransformOpt:     []string{"all,charmap=ISO-8859-7"},
			TransformBackOpt: []string{"all,charmap=ISO-8859-7"},
			Lossless:         false,
		}},
		{name: "lowercase", args: args{
			TransformOpt:     []string{"all,lowercase"},
			TransformBackOpt: []string{"all,lowercase"},
			Lossless:         false,
		}},
		{name: "ascii", args: args{
			TransformOpt:     []string{"all,ascii"},
			TransformBackOpt: []string{"all,ascii"},
			Lossless:         false,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fstest.NewRun(t)
			defer r.Finalise()

			ctx := context.Background()
			r.Mkdir(ctx, r.Flocal)
			r.Mkdir(ctx, r.Fremote)
			items := makeTestFiles(t, r, "dir1")
			err := r.Fremote.Mkdir(ctx, "empty/empty")
			require.NoError(t, err)
			err = r.Flocal.Mkdir(ctx, "empty/empty")
			require.NoError(t, err)
			deleteDSStore(t, r)
			r.CheckRemoteListing(t, items, []string{"dir1", "empty", "empty/empty"})
			r.CheckLocalListing(t, items, []string{"dir1", "empty", "empty/empty"})

			err = transform.SetOptions(ctx, tt.args.TransformOpt...)
			require.NoError(t, err)

			err = sync.Transform(ctx, r.Fremote, true, true)
			assert.NoError(t, err)
			compareNames(ctx, t, r, items)

			transformedItems := transformItems(ctx, t, items)
			r.CheckRemoteListing(t, transformedItems, []string{transform.Path(ctx, "dir1", true), transform.Path(ctx, "empty", true), transform.Path(ctx, "empty/empty", true)})
			err = transform.SetOptions(ctx, tt.args.TransformBackOpt...)
			require.NoError(t, err)
			err = sync.Transform(ctx, r.Fremote, true, true)
			assert.NoError(t, err)
			compareNames(ctx, t, r, transformedItems)

			if tt.args.Lossless {
				deleteDSStore(t, r)
				r.CheckRemoteListing(t, items, []string{"dir1", "empty", "empty/empty"})
			}
		})
	}
}

// const alphabet = "ƀɀɠʀҠԀڀڠݠހ߀ကႠᄀᄠᅀᆀᇠሀሠበዠጠᎠᏀᐠᑀᑠᒀᒠᓀᓠᔀᔠᕀᕠᖀᖠᗀᗠᘀᘠᙀᚠᛀកᠠᡀᣀᦀ᧠ᨠᯀᰀᴀ⇠⋀⍀⍠⎀⎠⏀␀─┠╀╠▀■◀◠☀☠♀♠⚀⚠⛀⛠✀✠❀➀➠⠀⠠⡀⡠⢀⢠⣀⣠⤀⤠⥀⥠⦠⨠⩀⪀⪠⫠⬀⬠⭀ⰀⲀⲠⳀⴀⵀ⺠⻀㇀㐀㐠㑀㑠㒀㒠㓀㓠㔀㔠㕀㕠㖀㖠㗀㗠㘀㘠㙀㙠㚀㚠㛀㛠㜀㜠㝀㝠㞀㞠㟀㟠㠀㠠㡀㡠㢀㢠㣀㣠㤀㤠㥀㥠㦀㦠㧀㧠㨀㨠㩀㩠㪀㪠㫀㫠㬀㬠㭀㭠㮀㮠㯀㯠㰀㰠㱀㱠㲀㲠㳀㳠㴀㴠㵀㵠㶀㶠㷀㷠㸀㸠㹀㹠㺀㺠㻀㻠㼀㼠㽀㽠㾀㾠㿀㿠䀀䀠䁀䁠䂀䂠䃀䃠䄀䄠䅀䅠䆀䆠䇀䇠䈀䈠䉀䉠䊀䊠䋀䋠䌀䌠䍀䍠䎀䎠䏀䏠䐀䐠䑀䑠䒀䒠䓀䓠䔀䔠䕀䕠䖀䖠䗀䗠䘀䘠䙀䙠䚀䚠䛀䛠䜀䜠䝀䝠䞀䞠䟀䟠䠀䠠䡀䡠䢀䢠䣀䣠䤀䤠䥀䥠䦀䦠䧀䧠䨀䨠䩀䩠䪀䪠䫀䫠䬀䬠䭀䭠䮀䮠䯀䯠䰀䰠䱀䱠䲀䲠䳀䳠䴀䴠䵀䵠䶀䷀䷠一丠乀习亀亠什仠伀传佀你侀侠俀俠倀倠偀偠傀傠僀僠儀儠兀兠冀冠净几刀删剀剠劀加勀勠匀匠區占厀厠叀叠吀吠呀呠咀咠哀哠唀唠啀啠喀喠嗀嗠嘀嘠噀噠嚀嚠囀因圀圠址坠垀垠埀埠堀堠塀塠墀墠壀壠夀夠奀奠妀妠姀姠娀娠婀婠媀媠嫀嫠嬀嬠孀孠宀宠寀寠尀尠局屠岀岠峀峠崀崠嵀嵠嶀嶠巀巠帀帠幀幠庀庠廀廠开张彀彠往徠忀忠怀怠恀恠悀悠惀惠愀愠慀慠憀憠懀懠戀戠所扠技抠拀拠挀挠捀捠掀掠揀揠搀搠摀摠撀撠擀擠攀攠敀敠斀斠旀无昀映晀晠暀暠曀曠最朠杀杠枀枠柀柠栀栠桀桠梀梠检棠椀椠楀楠榀榠槀槠樀樠橀橠檀檠櫀櫠欀欠歀歠殀殠毀毠氀氠汀池沀沠泀泠洀洠浀浠涀涠淀淠渀渠湀湠満溠滀滠漀漠潀潠澀澠激濠瀀瀠灀灠炀炠烀烠焀焠煀煠熀熠燀燠爀爠牀牠犀犠狀狠猀猠獀獠玀玠珀珠琀琠瑀瑠璀璠瓀瓠甀甠畀畠疀疠痀痠瘀瘠癀癠皀皠盀盠眀眠着睠瞀瞠矀矠砀砠础硠碀碠磀磠礀礠祀祠禀禠秀秠稀稠穀穠窀窠竀章笀笠筀筠简箠節篠簀簠籀籠粀粠糀糠紀素絀絠綀綠緀締縀縠繀繠纀纠绀绠缀缠罀罠羀羠翀翠耀耠聀聠肀肠胀胠脀脠腀腠膀膠臀臠舀舠艀艠芀芠苀苠茀茠荀荠莀莠菀菠萀萠葀葠蒀蒠蓀蓠蔀蔠蕀蕠薀薠藀藠蘀蘠虀虠蚀蚠蛀蛠蜀蜠蝀蝠螀螠蟀蟠蠀蠠血衠袀袠裀裠褀褠襀襠覀覠觀觠言訠詀詠誀誠諀諠謀謠譀譠讀讠诀诠谀谠豀豠貀負賀賠贀贠赀赠趀趠跀跠踀踠蹀蹠躀躠軀軠輀輠轀轠辀辠迀迠退造遀遠邀邠郀郠鄀鄠酀酠醀醠釀釠鈀鈠鉀鉠銀銠鋀鋠錀錠鍀鍠鎀鎠鏀鏠鐀鐠鑀鑠钀钠铀铠销锠镀镠門閠闀闠阀阠陀陠隀隠雀雠需霠靀靠鞀鞠韀韠頀頠顀顠颀颠飀飠餀餠饀饠馀馠駀駠騀騠驀驠骀骠髀髠鬀鬠魀魠鮀鮠鯀鯠鰀鰠鱀鱠鲀鲠鳀鳠鴀鴠鵀鵠鶀鶠鷀鷠鸀鸠鹀鹠麀麠黀黠鼀鼠齀齠龀龠ꀀꀠꁀꁠꂀꂠꃀꃠꄀꄠꅀꅠꆀꆠꇀꇠꈀꈠꉀꉠꊀꊠꋀꋠꌀꌠꍀꍠꎀꎠꏀꏠꐀꐠꑀꑠ꒠ꔀꔠꕀꕠꖀꖠꗀꗠꙀꚠꛀ꜀꜠ꝀꞀꡀ測試_Русский___ě_áñ"
const alphabet = "abcdefg123456789Ü"

var extras = []string{"apple", "banana", "appleappleapplebanana", "splitbananasplit"}

func makeTestFiles(t *testing.T, r *fstest.Run, dir string) []fstest.Item {
	t.Helper()
	n := 0
	// Create test files
	items := []fstest.Item{}
	for _, c := range alphabet {
		var out strings.Builder
		for i := range rune(7) {
			out.WriteRune(c + i)
		}
		fileName := path.Join(dir, fmt.Sprintf("%04d-%s.txt", n, out.String()))
		fileName = strings.ToValidUTF8(fileName, "")
		fileName = strings.NewReplacer(":", "", "<", "", ">", "", "?", "").Replace(fileName) // remove characters illegal on windows

		if debug != "" {
			fileName = debug
		}

		item := r.WriteObject(context.Background(), fileName, fileName, t1)
		r.WriteFile(fileName, fileName, t1)
		items = append(items, item)
		n++

		if debug != "" {
			break
		}
	}

	for _, extra := range extras {
		item := r.WriteObject(context.Background(), extra, extra, t1)
		r.WriteFile(extra, extra, t1)
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

func compareNames(ctx context.Context, t *testing.T, r *fstest.Run, items []fstest.Item) {
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
		aConv := transform.Path(ctx, a.Path, false)
		bConv := transform.Path(ctx, b.Path, false)
		return cmp.Compare(aConv, bConv)
	})
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Remote(), b.Remote())
	})

	for i, e := range entries {
		expect := transform.Path(ctx, items[i].Path, false)
		msg := fmt.Sprintf("expected %v, got %v", detectEncoding(expect), detectEncoding(e.Remote()))
		assert.Equal(t, expect, e.Remote(), msg)
	}
}

func transformItems(ctx context.Context, t *testing.T, items []fstest.Item) []fstest.Item {
	transformedItems := []fstest.Item{}
	for _, item := range items {
		newPath := transform.Path(ctx, item.Path, false)
		newItem := item
		newItem.Path = newPath
		transformedItems = append(transformedItems, newItem)
	}
	return transformedItems
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

func TestUnicodeEquivalence(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx := context.Background()
	r.Mkdir(ctx, r.Fremote)
	const remote = "Über"
	item := r.WriteObject(ctx, remote, "", t1)

	obj, err := r.Fremote.NewObject(ctx, remote) // can't use r.CheckRemoteListing here as it forces NFC
	require.NoError(t, err)
	require.NotEmpty(t, obj)

	err = transform.SetOptions(ctx, "all,nfc")
	require.NoError(t, err)

	err = sync.Transform(ctx, r.Fremote, true, true)
	assert.NoError(t, err)
	item.Path = norm.NFC.String(item.Path)
	r.CheckRemoteListing(t, []fstest.Item{item}, nil)
}
