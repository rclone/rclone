package base32768

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

/*
 * Encodings
 */

const blockBit = 5
const safeAlphabet = "ƀɀɠʀҠԀڀڠݠހ߀ကႠᄀᄠᅀᆀᇠሀሠበዠጠᎠᏀᐠᑀᑠᒀᒠᓀᓠᔀᔠᕀᕠᖀᖠᗀᗠᘀᘠᙀᚠᛀកᠠᡀᣀᦀ᧠ᨠᯀᰀᴀ⇠⋀⍀⍠⎀⎠⏀␀─┠╀╠▀■◀◠☀☠♀♠⚀⚠⛀⛠✀✠❀➀➠⠀⠠⡀⡠⢀⢠⣀⣠⤀⤠⥀⥠⦠⨠⩀⪀⪠⫠⬀⬠⭀ⰀⲀⲠⳀⴀⵀ⺠⻀㇀㐀㐠㑀㑠㒀㒠㓀㓠㔀㔠㕀㕠㖀㖠㗀㗠㘀㘠㙀㙠㚀㚠㛀㛠㜀㜠㝀㝠㞀㞠㟀㟠㠀㠠㡀㡠㢀㢠㣀㣠㤀㤠㥀㥠㦀㦠㧀㧠㨀㨠㩀㩠㪀㪠㫀㫠㬀㬠㭀㭠㮀㮠㯀㯠㰀㰠㱀㱠㲀㲠㳀㳠㴀㴠㵀㵠㶀㶠㷀㷠㸀㸠㹀㹠㺀㺠㻀㻠㼀㼠㽀㽠㾀㾠㿀㿠䀀䀠䁀䁠䂀䂠䃀䃠䄀䄠䅀䅠䆀䆠䇀䇠䈀䈠䉀䉠䊀䊠䋀䋠䌀䌠䍀䍠䎀䎠䏀䏠䐀䐠䑀䑠䒀䒠䓀䓠䔀䔠䕀䕠䖀䖠䗀䗠䘀䘠䙀䙠䚀䚠䛀䛠䜀䜠䝀䝠䞀䞠䟀䟠䠀䠠䡀䡠䢀䢠䣀䣠䤀䤠䥀䥠䦀䦠䧀䧠䨀䨠䩀䩠䪀䪠䫀䫠䬀䬠䭀䭠䮀䮠䯀䯠䰀䰠䱀䱠䲀䲠䳀䳠䴀䴠䵀䵠䶀䷀䷠一丠乀习亀亠什仠伀传佀你侀侠俀俠倀倠偀偠傀傠僀僠儀儠兀兠冀冠净几刀删剀剠劀加勀勠匀匠區占厀厠叀叠吀吠呀呠咀咠哀哠唀唠啀啠喀喠嗀嗠嘀嘠噀噠嚀嚠囀因圀圠址坠垀垠埀埠堀堠塀塠墀墠壀壠夀夠奀奠妀妠姀姠娀娠婀婠媀媠嫀嫠嬀嬠孀孠宀宠寀寠尀尠局屠岀岠峀峠崀崠嵀嵠嶀嶠巀巠帀帠幀幠庀庠廀廠开张彀彠往徠忀忠怀怠恀恠悀悠惀惠愀愠慀慠憀憠懀懠戀戠所扠技抠拀拠挀挠捀捠掀掠揀揠搀搠摀摠撀撠擀擠攀攠敀敠斀斠旀无昀映晀晠暀暠曀曠最朠杀杠枀枠柀柠栀栠桀桠梀梠检棠椀椠楀楠榀榠槀槠樀樠橀橠檀檠櫀櫠欀欠歀歠殀殠毀毠氀氠汀池沀沠泀泠洀洠浀浠涀涠淀淠渀渠湀湠満溠滀滠漀漠潀潠澀澠激濠瀀瀠灀灠炀炠烀烠焀焠煀煠熀熠燀燠爀爠牀牠犀犠狀狠猀猠獀獠玀玠珀珠琀琠瑀瑠璀璠瓀瓠甀甠畀畠疀疠痀痠瘀瘠癀癠皀皠盀盠眀眠着睠瞀瞠矀矠砀砠础硠碀碠磀磠礀礠祀祠禀禠秀秠稀稠穀穠窀窠竀章笀笠筀筠简箠節篠簀簠籀籠粀粠糀糠紀素絀絠綀綠緀締縀縠繀繠纀纠绀绠缀缠罀罠羀羠翀翠耀耠聀聠肀肠胀胠脀脠腀腠膀膠臀臠舀舠艀艠芀芠苀苠茀茠荀荠莀莠菀菠萀萠葀葠蒀蒠蓀蓠蔀蔠蕀蕠薀薠藀藠蘀蘠虀虠蚀蚠蛀蛠蜀蜠蝀蝠螀螠蟀蟠蠀蠠血衠袀袠裀裠褀褠襀襠覀覠觀觠言訠詀詠誀誠諀諠謀謠譀譠讀讠诀诠谀谠豀豠貀負賀賠贀贠赀赠趀趠跀跠踀踠蹀蹠躀躠軀軠輀輠轀轠辀辠迀迠退造遀遠邀邠郀郠鄀鄠酀酠醀醠釀釠鈀鈠鉀鉠銀銠鋀鋠錀錠鍀鍠鎀鎠鏀鏠鐀鐠鑀鑠钀钠铀铠销锠镀镠門閠闀闠阀阠陀陠隀隠雀雠需霠靀靠鞀鞠韀韠頀頠顀顠颀颠飀飠餀餠饀饠馀馠駀駠騀騠驀驠骀骠髀髠鬀鬠魀魠鮀鮠鯀鯠鰀鰠鱀鱠鲀鲠鳀鳠鴀鴠鵀鵠鶀鶠鷀鷠鸀鸠鹀鹠麀麠黀黠鼀鼠齀齠龀龠ꀀꀠꁀꁠꂀꂠꃀꃠꄀꄠꅀꅠꆀꆠꇀꇠꈀꈠꉀꉠꊀꊠꋀꋠꌀꌠꍀꍠꎀꎠꏀꏠꐀꐠꑀꑠ꒠ꔀꔠꕀꕠꖀꖠꗀꗠꙀꚠꛀ꜀꜠ꝀꞀꡀ"
const shortAlphabet = "ÀàĀĠŀŠƀƠǀǠȀȠɀɠʀʠˀˠ̠̀̀͠΀ΠπϠЀРрѠҀҠӀӠԀԠՀՠր֠׀נ؀ؠـ٠ڀڠۀ۠܀ܠ݀ݠހޠ߀ߠዠጠᎠᏀᐠᑀᑠᒀᒠᓀᓠᔀᔠᕀᕠᖀᖠᗀᗠᘀᘠᙀᚠᛀកᠠᡀᣀ─┠╀╠▀■◀◠☀☠♀♠⚀⚠⛀⛠✀✠❀➀➠⠠⡀⡠⢀⢠⣀⣠⫠⬀⬠⭀ⰀⲀⲠⳀⴀⵀ㐀㐠㑀㑠㒀㒠㓀㓠㔀㔠㕀㕠㖀㖠㗀㗠㘀㘠㙀㙠㚀㚠㛀㛠㜀㜠㝀㝠㞀㞠㟀㟠㠀㠠㡀㡠㢀㢠㣀㣠㤀㤠㥀㥠㦀㦠㧀㧠㨀㨠㩀㩠㪀㪠㫀㫠㬀㬠㭀㭠㮀㮠㯀㯠㰀㰠㱀㱠㲀㲠㳀㳠㴀㴠㵀㵠㶀㶠㷀㷠㸀㸠㹀㹠㺀㺠㻀㻠㼀㼠㽀㽠㾀㾠㿀㿠䀀䀠䁀䁠䂀䂠䃀䃠䄀䄠䅀䅠䆀䆠䇀䇠䈀䈠䉀䉠䊀䊠䋀䋠䌀䌠䍀䍠䎀䎠䏀䏠䐀䐠䑀䑠䒀䒠䓀䓠䔀䔠䕀䕠䖀䖠䗀䗠䘀䘠䙀䙠䚀䚠䛀䛠䜀䜠䝀䝠䞀䞠䟀䟠䠀䠠䡀䡠䢀䢠䣀䣠䤀䤠䥀䥠䦀䦠䧀䧠䨀䨠䩀䩠䪀䪠䫀䫠䬀䬠䭀䭠䮀䮠䯀䯠䰀䰠䱀䱠䲀䲠䳀䳠䴀䴠䵀䵠䶀䷀䷠一丠乀习亀亠什仠伀传佀你侀侠俀俠倀倠偀偠傀傠僀僠儀儠兀兠冀冠净几刀删剀剠劀加勀勠匀匠區占厀厠叀叠吀吠呀呠咀咠哀哠唀唠啀啠喀喠嗀嗠嘀嘠噀噠嚀嚠囀因圀圠址坠垀垠埀埠堀堠塀塠墀墠壀壠夀夠奀奠妀妠姀姠娀娠婀婠媀媠嫀嫠嬀嬠孀孠宀宠寀寠尀尠局屠岀岠峀峠崀崠嵀嵠嶀嶠巀巠帀帠幀幠庀庠廀廠开张彀彠往徠忀忠怀怠恀恠悀悠惀惠愀愠慀慠憀憠懀懠戀戠所扠技抠拀拠挀挠捀捠掀掠揀揠搀搠摀摠撀撠擀擠攀攠敀敠斀斠旀无昀映晀晠暀暠曀曠最朠杀杠枀枠柀柠栀栠桀桠梀梠检棠椀椠楀楠榀榠槀槠樀樠橀橠檀檠櫀櫠欀欠歀歠殀殠毀毠氀氠汀池沀沠泀泠洀洠浀浠涀涠淀淠渀渠湀湠満溠滀滠漀漠潀潠澀澠激濠瀀瀠灀灠炀炠烀烠焀焠煀煠熀熠燀燠爀爠牀牠犀犠狀狠猀猠獀獠玀玠珀珠琀琠瑀瑠璀璠瓀瓠甀甠畀畠疀疠痀痠瘀瘠癀癠皀皠盀盠眀眠着睠瞀瞠矀矠砀砠础硠碀碠磀磠礀礠祀祠禀禠秀秠稀稠穀穠窀窠竀章笀笠筀筠简箠節篠簀簠籀籠粀粠糀糠紀素絀絠綀綠緀締縀縠繀繠纀纠绀绠缀缠罀罠羀羠翀翠耀耠聀聠肀肠胀胠脀脠腀腠膀膠臀臠舀舠艀艠芀芠苀苠茀茠荀荠莀莠菀菠萀萠葀葠蒀蒠蓀蓠蔀蔠蕀蕠薀薠藀藠蘀蘠虀虠蚀蚠蛀蛠蜀蜠蝀蝠螀螠蟀蟠蠀蠠血衠袀袠裀裠褀褠襀襠覀覠觀觠言訠詀詠誀誠諀諠謀謠譀譠讀讠诀诠谀谠豀豠貀負賀賠贀贠赀赠趀趠跀跠踀踠蹀蹠躀躠軀軠輀輠轀轠辀辠迀迠退造遀遠邀邠郀郠鄀鄠酀酠醀醠釀釠鈀鈠鉀鉠銀銠鋀鋠錀錠鍀鍠鎀鎠鏀鏠鐀鐠鑀鑠钀钠铀铠销锠镀镠門閠闀闠阀阠陀陠隀隠雀雠需霠靀靠鞀鞠韀韠頀頠顀顠颀颠飀飠餀餠饀饠馀馠駀駠騀騠驀驠骀骠髀髠鬀鬠魀魠鮀鮠鯀鯠鰀鰠鱀鱠鲀鲠鳀鳠鴀鴠鵀鵠鶀鶠鷀鷠鸀鸠鹀鹠麀麠黀黠鼀鼠齀齠龀龠ꀀꀠꁀꁠꂀꂠꃀꃠꄀꄠꅀꅠꆀꆠꇀꇠꈀꈠꉀꉠꊀꊠꋀꋠꌀꌠꍀꍠꎀꎠꏀꏠꐀꐠꑀꑠ꒠ꔀꔠꕀꕠꖀꖠ"
const safeAlphabetCI = "\u0260\u0280\u0560\u0680\u06A0\u0760\u0780\u07C0\u08A0\u1000\u1100\u1120\u1140\u1180\u11E0\u1200\u1220\u1260\u12E0\u1320\u1420\u1440\u1460\u1480\u14A0\u14C0\u14E0\u1500\u1520\u1540\u1560\u1580\u15A0\u15C0\u15E0\u1600\u1620\u1640\u16A0\u16C0\u1780\u1820\u1840\u18C0\u1980\u19E0\u1A20\u1BC0\u1C00\u1D00\u21E0\u22C0\u2340\u2360\u2380\u23A0\u23C0\u23E0\u2400\u2500\u2520\u2540\u2560\u2580\u25A0\u25C0\u25E0\u2600\u2620\u2640\u2660\u2680\u26A0\u26C0\u26E0\u2700\u2720\u2740\u2780\u27A0\u2820\u2840\u2860\u2880\u28A0\u28C0\u28E0\u2900\u2920\u2940\u2960\u29A0\u2A20\u2A40\u2A80\u2AA0\u2AE0\u2B00\u2B20\u2B40\u2BA0\u2BC0\u2BE0\u2C40\u2D00\u2D40\u2EA0\u2EC0\u31A0\u31C0\u3400\u3420\u3440\u3460\u3480\u34A0\u34C0\u34E0\u3500\u3520\u3540\u3560\u3580\u35A0\u35C0\u35E0\u3600\u3620\u3640\u3660\u3680\u36A0\u36C0\u36E0\u3700\u3720\u3740\u3760\u3780\u37A0\u37C0\u37E0\u3800\u3820\u3840\u3860\u3880\u38A0\u38C0\u38E0\u3900\u3920\u3940\u3960\u3980\u39A0\u39C0\u39E0\u3A00\u3A20\u3A40\u3A60\u3A80\u3AA0\u3AC0\u3AE0\u3B00\u3B20\u3B40\u3B60\u3B80\u3BA0\u3BC0\u3BE0\u3C00\u3C20\u3C40\u3C60\u3C80\u3CA0\u3CC0\u3CE0\u3D00\u3D20\u3D40\u3D60\u3D80\u3DA0\u3DC0\u3DE0\u3E00\u3E20\u3E40\u3E60\u3E80\u3EA0\u3EC0\u3EE0\u3F00\u3F20\u3F40\u3F60\u3F80\u3FA0\u3FC0\u3FE0\u4000\u4020\u4040\u4060\u4080\u40A0\u40C0\u40E0\u4100\u4120\u4140\u4160\u4180\u41A0\u41C0\u41E0\u4200\u4220\u4240\u4260\u4280\u42A0\u42C0\u42E0\u4300\u4320\u4340\u4360\u4380\u43A0\u43C0\u43E0\u4400\u4420\u4440\u4460\u4480\u44A0\u44C0\u44E0\u4500\u4520\u4540\u4560\u4580\u45A0\u45C0\u45E0\u4600\u4620\u4640\u4660\u4680\u46A0\u46C0\u46E0\u4700\u4720\u4740\u4760\u4780\u47A0\u47C0\u47E0\u4800\u4820\u4840\u4860\u4880\u48A0\u48C0\u48E0\u4900\u4920\u4940\u4960\u4980\u49A0\u49C0\u49E0\u4A00\u4A20\u4A40\u4A60\u4A80\u4AA0\u4AC0\u4AE0\u4B00\u4B20\u4B40\u4B60\u4B80\u4BA0\u4BC0\u4BE0\u4C00\u4C20\u4C40\u4C60\u4C80\u4CA0\u4CC0\u4CE0\u4D00\u4D20\u4D40\u4D60\u4D80\u4DA0\u4DC0\u4DE0\u4E00\u4E20\u4E40\u4E60\u4E80\u4EA0\u4EC0\u4EE0\u4F00\u4F20\u4F40\u4F60\u4F80\u4FA0\u4FC0\u4FE0\u5000\u5020\u5040\u5060\u5080\u50A0\u50C0\u50E0\u5100\u5120\u5140\u5160\u5180\u51A0\u51C0\u51E0\u5200\u5220\u5240\u5260\u5280\u52A0\u52C0\u52E0\u5300\u5320\u5340\u5360\u5380\u53A0\u53C0\u53E0\u5400\u5420\u5440\u5460\u5480\u54A0\u54C0\u54E0\u5500\u5520\u5540\u5560\u5580\u55A0\u55C0\u55E0\u5600\u5620\u5640\u5660\u5680\u56A0\u56C0\u56E0\u5700\u5720\u5740\u5760\u5780\u57A0\u57C0\u57E0\u5800\u5820\u5840\u5860\u5880\u58A0\u58C0\u58E0\u5900\u5920\u5940\u5960\u5980\u59A0\u59C0\u59E0\u5A00\u5A20\u5A40\u5A60\u5A80\u5AA0\u5AC0\u5AE0\u5B00\u5B20\u5B40\u5B60\u5B80\u5BA0\u5BC0\u5BE0\u5C00\u5C20\u5C40\u5C60\u5C80\u5CA0\u5CC0\u5CE0\u5D00\u5D20\u5D40\u5D60\u5D80\u5DA0\u5DC0\u5DE0\u5E00\u5E20\u5E40\u5E60\u5E80\u5EA0\u5EC0\u5EE0\u5F00\u5F20\u5F40\u5F60\u5F80\u5FA0\u5FC0\u5FE0\u6000\u6020\u6040\u6060\u6080\u60A0\u60C0\u60E0\u6100\u6120\u6140\u6160\u6180\u61A0\u61C0\u61E0\u6200\u6220\u6240\u6260\u6280\u62A0\u62C0\u62E0\u6300\u6320\u6340\u6360\u6380\u63A0\u63C0\u63E0\u6400\u6420\u6440\u6460\u6480\u64A0\u64C0\u64E0\u6500\u6520\u6540\u6560\u6580\u65A0\u65C0\u65E0\u6600\u6620\u6640\u6660\u6680\u66A0\u66C0\u66E0\u6700\u6720\u6740\u6760\u6780\u67A0\u67C0\u67E0\u6800\u6820\u6840\u6860\u6880\u68A0\u68C0\u68E0\u6900\u6920\u6940\u6960\u6980\u69A0\u69C0\u69E0\u6A00\u6A20\u6A40\u6A60\u6A80\u6AA0\u6AC0\u6AE0\u6B00\u6B20\u6B40\u6B60\u6B80\u6BA0\u6BC0\u6BE0\u6C00\u6C20\u6C40\u6C60\u6C80\u6CA0\u6CC0\u6CE0\u6D00\u6D20\u6D40\u6D60\u6D80\u6DA0\u6DC0\u6DE0\u6E00\u6E20\u6E40\u6E60\u6E80\u6EA0\u6EC0\u6EE0\u6F00\u6F20\u6F40\u6F60\u6F80\u6FA0\u6FC0\u6FE0\u7000\u7020\u7040\u7060\u7080\u70A0\u70C0\u70E0\u7100\u7120\u7140\u7160\u7180\u71A0\u71C0\u71E0\u7200\u7220\u7240\u7260\u7280\u72A0\u72C0\u72E0\u7300\u7320\u7340\u7360\u7380\u73A0\u73C0\u73E0\u7400\u7420\u7440\u7460\u7480\u74A0\u74C0\u74E0\u7500\u7520\u7540\u7560\u7580\u75A0\u75C0\u75E0\u7600\u7620\u7640\u7660\u7680\u76A0\u76C0\u76E0\u7700\u7720\u7740\u7760\u7780\u77A0\u77C0\u77E0\u7800\u7820\u7840\u7860\u7880\u78A0\u78C0\u78E0\u7900\u7920\u7940\u7960\u7980\u79A0\u79C0\u79E0\u7A00\u7A20\u7A40\u7A60\u7A80\u7AA0\u7AC0\u7AE0\u7B00\u7B20\u7B40\u7B60\u7B80\u7BA0\u7BC0\u7BE0\u7C00\u7C20\u7C40\u7C60\u7C80\u7CA0\u7CC0\u7CE0\u7D00\u7D20\u7D40\u7D60\u7D80\u7DA0\u7DC0\u7DE0\u7E00\u7E20\u7E40\u7E60\u7E80\u7EA0\u7EC0\u7EE0\u7F00\u7F20\u7F40\u7F60\u7F80\u7FA0\u7FC0\u7FE0\u8000\u8020\u8040\u8060\u8080\u80A0\u80C0\u80E0\u8100\u8120\u8140\u8160\u8180\u81A0\u81C0\u81E0\u8200\u8220\u8240\u8260\u8280\u82A0\u82C0\u82E0\u8300\u8320\u8340\u8360\u8380\u83A0\u83C0\u83E0\u8400\u8420\u8440\u8460\u8480\u84A0\u84C0\u84E0\u8500\u8520\u8540\u8560\u8580\u85A0\u85C0\u85E0\u8600\u8620\u8640\u8660\u8680\u86A0\u86C0\u86E0\u8700\u8720\u8740\u8760\u8780\u87A0\u87C0\u87E0\u8800\u8820\u8840\u8860\u8880\u88A0\u88C0\u88E0\u8900\u8920\u8940\u8960\u8980\u89A0\u89C0\u89E0\u8A00\u8A20\u8A40\u8A60\u8A80\u8AA0\u8AC0\u8AE0\u8B00\u8B20\u8B40\u8B60\u8B80\u8BA0\u8BC0\u8BE0\u8C00\u8C20\u8C40\u8C60\u8C80\u8CA0\u8CC0\u8CE0\u8D00\u8D20\u8D40\u8D60\u8D80\u8DA0\u8DC0\u8DE0\u8E00\u8E20\u8E40\u8E60\u8E80\u8EA0\u8EC0\u8EE0\u8F00\u8F20\u8F40\u8F60\u8F80\u8FA0\u8FC0\u8FE0\u9000\u9020\u9040\u9060\u9080\u90A0\u90C0\u90E0\u9100\u9120\u9140\u9160\u9180\u91A0\u91C0\u91E0\u9200\u9220\u9240\u9260\u9280\u92A0\u92C0\u92E0\u9300\u9320\u9340\u9360\u9380\u93A0\u93C0\u93E0\u9400\u9420\u9440\u9460\u9480\u94A0\u94C0\u94E0\u9500\u9520\u9540\u9560\u9580\u95A0\u95C0\u95E0\u9600\u9620\u9640\u9660\u9680\u96A0\u96C0\u96E0\u9700\u9720\u9740\u9760\u9780\u97A0\u97C0\u97E0\u9800\u9820\u9840\u9860\u9880\u98A0\u98C0\u98E0\u9900\u9920\u9940\u9960\u9980\u99A0\u99C0\u99E0\u9A00\u9A20\u9A40\u9A60\u9A80\u9AA0\u9AC0\u9AE0\u9B00\u9B20\u9B40\u9B60\u9B80\u9BA0\u9BC0\u9BE0\u9C00\u9C20\u9C40\u9C60\u9C80\u9CA0\u9CC0\u9CE0\u9D00\u9D20\u9D40\u9D60\u9D80\u9DA0\u9DC0\u9DE0\u9E00\u9E20\u9E40\u9E60\u9E80\u9EA0\u9EC0\u9EE0\u9F00\u9F20\u9F40\u9F60\u9F80\u9FA0\u9FC0\u9FE0\uA000\uA020\uA040\uA060\uA080\uA0A0\uA0C0\uA0E0\uA100\uA120\uA140\uA160\uA180\uA1A0\uA1C0\uA1E0\uA200\uA220\uA240\uA260\uA280\uA2A0\uA2C0\uA2E0\uA300\uA320\uA340\uA360\uA380\uA3A0\uA3C0\uA3E0\uA400\uA420\uA440\uA460\uA4A0\uA500\uA520\uA540\uA560\uA580\uA5A0\uA5C0\uA5E0\uA6A0\uA6C0\uA700\uA840\uA900\uAA00\uAA80\uAB80\uABA0"

// An Encoding is a radix 32768 encoding/decoding scheme, defined by a
// 32768-character alphabet.
type Encoding struct {
	encodeA   [1024]uint16
	encodeB   [4]uint16
	decodeMap [2048]uint16
	splitter  uint16
}

// NewEncoding returns a new Encoding defined by the given alphabet,
// The alphabet must be a 1028 characters long and contains only BMP
// character and 32 block leading characters.
func NewEncoding(encoder string) *Encoding {
	e := new(Encoding)
	encode := make([]uint16, 1028)
	i := 0
	for _, r := range encoder {
		if r&0xFFE0 != r {
			panic("encoding alphabet containing illegal character")
		}
		if i >= len(encode) {
			break
		}
		encode[i] = uint16(r)
		i++
	}
	if i < len(encode) {
		panic("encoding alphabet is not 1028-characters long")
	}
	sort.Slice(encode, func(i, j int) bool { return encode[i] < encode[j] })
	e.splitter = encode[4]
	copy(e.encodeA[:], encode[4:])
	copy(e.encodeB[:], encode[:4])

	for i := 0; i < len(e.decodeMap); i++ {
		e.decodeMap[i] = 0xFFFD
	}
	for i := 0; i < len(e.encodeA); i++ {
		idx := e.encodeA[i] >> blockBit
		if e.decodeMap[idx] != 0xFFFD {
			panic("encoding alphabet have repeating character")
		}
		e.decodeMap[idx] = uint16(i) << blockBit
	}
	for i := 0; i < len(e.encodeB); i++ {
		idx := e.encodeB[i] >> blockBit
		if e.decodeMap[idx] != 0xFFFD {
			panic("encoding alphabet have repeating character")
		}
		e.decodeMap[idx] = uint16(i) << blockBit
	}
	return e
}

// SafeEncoding is a base32768 encoding using only "safe"
// Unicode runes
var SafeEncoding = NewEncoding(safeAlphabet)
// ShortEncoding is a base32768 encoding use all codepoint
// from 0x00C0 - 0x0800 to optimize UTF-8 behavior
var ShortEncoding = NewEncoding(shortAlphabet)
// SafeEncodingCI is a base32768 encoding using only case insensitive "safe"
// Unicode runes
var SafeEncodingCI = NewEncoding(safeAlphabetCI)



var removeNewlinesMapper = func(r rune) rune {
	if r == '\r' || r == '\n' {
		return -1
	}
	return r
}

func (enc *Encoding) encode15(src uint16) uint16 {
	src = src & 0x7FFF
	dst := enc.encodeA[src>>blockBit]
	dst |= uint16(src & (1<<blockBit - 1))
	return dst
}

func (enc *Encoding) encode7(src byte) uint16 {
	src = src & 0x7F
	dst := enc.encodeB[src>>blockBit]
	dst |= uint16(src & (1<<blockBit - 1))
	return dst
}

func (enc *Encoding) decode(src uint16) (uint16, bool, bool) {
	isTrailing := src < enc.splitter
	dst := enc.decodeMap[src>>blockBit]
	if dst == 0xFFFD {
		return dst, isTrailing, false
	}
	dst |= src & (1<<blockBit - 1)
	return dst, isTrailing, true
}

func (enc *Encoding) encodeUint16(dst []uint16, src []byte) {
	var left byte
	var leftn uint8
	for len(src) > 0 {
		var chunk uint16 // Chunk contains 15 bits
		chunk = uint16(left) << (15 - leftn)
		chunk |= uint16(src[0]) << (7 - leftn)
		if leftn < 7 && len(src) > 1 {
			chunk |= uint16(src[1]) >> (1 + leftn)
			left = src[1] & (1<<(1+leftn) - 1)
			leftn++
			src = src[2:] // 2 bytes taken
		} else {
			chunk |= 1<<(7-leftn) - 1 // Pad with 1s
			left = 0
			leftn = 0
			src = src[1:] // 1 byte taken
		}
		dst[0] = enc.encode15(chunk)
		dst = dst[1:]
	}
	// Remaining
	if leftn > 0 {
		left = left << (7 - leftn)
		left |= 1<<(7-leftn) - 1 // Pad with 1s
		dst[0] = enc.encode7(left)
	}
}

// Encode encodes src using the encoding enc, writing
// EncodedLen(len(src)) bytes to dst.
func (enc *Encoding) Encode(dst, src []byte) {
	buf := make([]uint16, enc.EncodedLen(len(src))/2)
	enc.encodeUint16(buf, src)
	for _, b := range buf {
		if len(dst) > 1 {
			dst[0] = byte(b >> 8)
			dst[1] = byte(b)
			dst = dst[2:]
		}
	}
}

// EncodeToString returns the base32768 encoding of src.
func (enc *Encoding) EncodeToString(src []byte) string {
	buf := make([]uint16, enc.EncodedLen(len(src))/2)
	enc.encodeUint16(buf, src)
	return string(utf16.Decode(buf))
}

// EncodedLen returns the length in bytes of the base32768 encoding
// of an input buffer of length n.
func (enc *Encoding) EncodedLen(n int) int {
	return (8*n + 14) / 15 * 2
}

type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "illegal base32768 data at input byte " + strconv.FormatInt(int64(e*2), 10)
}

func (enc *Encoding) decodeUint16(dst []byte, src []uint16) (n int, err error) {
	olen := len(src)
	var left byte
	var leftn uint8
	for len(src) > 0 {
		chunk := src[0]
		d, trailing, success := enc.decode(chunk)
		if !success {
			return n, CorruptInputError(olen - len(src))
		}
		if trailing {
			// Left one byte
			if leftn > 0 {
				dst[0] = left<<(8-leftn) | byte(d>>(leftn-1))
				n++
			}
			return n, nil
		}
		// Read 15 bits
		if leftn > 0 {
			dst[0] = left<<(8-leftn) | byte(d>>(7+leftn))
			dst[1] = byte(d >> (leftn - 1))
			left = byte(d) & (1<<(leftn-1) - 1)
			leftn--
			dst = dst[2:]
			n += 2
		} else {
			dst[0] = byte(d >> 7)
			left = byte(d) & 0x7F
			leftn = 7
			dst = dst[1:]
			n++
		}
		src = src[1:]
	}
	return n, nil
}

// Decode decodes src using the encoding enc. It writes at most
// DecodedLen(len(src)) bytes to dst and returns the number of bytes
// written. If src contains invalid base32768 data, it will return the
// number of bytes successfully written and CorruptInputError.
// New line characters (\r and \n) are ignored.
func (enc *Encoding) Decode(dst, src []byte) (n int, err error) {
	if len(src)%2 != 0 {
		return 0, CorruptInputError(0)
	}
	buf := make([]uint16, len(src)/2)
	for i := range buf {
		if len(src) > 1 {
			buf[i] = uint16(src[0])<<8 | uint16(src[1])
			src = src[2:]
		}
	}
	return enc.decodeUint16(dst, buf)
}

// DecodeString returns the bytes represented by the base32768 string s.
func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	s = strings.Map(removeNewlinesMapper, s)
	src := utf16.Encode([]rune(s))
	dbuf := make([]byte, enc.DecodedLen(len(src)*2))
	n, err := enc.decodeUint16(dbuf, src)
	return dbuf[:n], err
}

// DecodedLen returns the maximum length in bytes of the decoded data
// corresponding to n bytes of base32768-encoded data.
func (enc *Encoding) DecodedLen(n int) int {
	return n / 2 * 15 / 8
}
