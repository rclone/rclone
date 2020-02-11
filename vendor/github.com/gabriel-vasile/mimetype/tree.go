package mimetype

import "github.com/gabriel-vasile/mimetype/internal/matchers"

// root is a matcher which passes for any slice of bytes.
// When a matcher passes the check, the children matchers
// are tried in order to find a more accurate MIME type.
var root = newMIME("application/octet-stream", "", matchers.True,
	sevenZ, zip, pdf, ole, ps, psd, ogg, png, jpg, jp2, jpx, jpm, gif, webp,
	exe, elf, ar, tar, xar, bz2, fits, tiff, bmp, ico, mp3, flac, midi, ape,
	musePack, amr, wav, aiff, au, mpeg, quickTime, mqv, mp4, webM, threeGP,
	threeG2, avi, flv, mkv, asf, aac, voc, aMp4, m4a, utf32le, utf32be, utf16le,
	utf16be, gzip, class, swf, crx, woff, woff2, otf, eot, wasm, shx, dbf, dcm,
	rar, djvu, mobi, lit, bpg, sqlite3, dwg, nes, macho, qcp, icns, heic,
	heicSeq, heif, heifSeq, mrc, mdb, accdb, zstd, cab, utf8,
)

// The list of nodes appended to the root node
var (
	gzip = newMIME("application/gzip", ".gz", matchers.Gzip).
		alias("application/x-gzip", "application/x-gunzip", "application/gzipped", "application/gzip-compressed", "application/x-gzip-compressed", "gzip/document")
	sevenZ = newMIME("application/x-7z-compressed", ".7z", matchers.SevenZ)
	zip    = newMIME("application/zip", ".zip", matchers.Zip, xlsx, docx, pptx, epub, jar, odt, ods, odp, odg, odf).
		alias("application/x-zip", "application/x-zip-compressed")
	tar = newMIME("application/x-tar", ".tar", matchers.Tar)
	xar = newMIME("application/x-xar", ".xar", matchers.Xar)
	bz2 = newMIME("application/x-bzip2", ".bz2", matchers.Bz2)
	pdf = newMIME("application/pdf", ".pdf", matchers.Pdf).
		alias("application/x-pdf")
	xlsx = newMIME("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx", matchers.Xlsx)
	docx = newMIME("application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx", matchers.Docx)
	pptx = newMIME("application/vnd.openxmlformats-officedocument.presentationml.presentation", ".pptx", matchers.Pptx)
	epub = newMIME("application/epub+zip", ".epub", matchers.Epub)
	jar  = newMIME("application/jar", ".jar", matchers.Jar)
	ole  = newMIME("application/x-ole-storage", "", matchers.Ole, xls, pub, ppt, doc)
	doc  = newMIME("application/msword", ".doc", matchers.Doc).
		alias("application/vnd.ms-word")
	ppt = newMIME("application/vnd.ms-powerpoint", ".ppt", matchers.Ppt).
		alias("application/mspowerpoint")
	pub = newMIME("application/vnd.ms-publisher", ".pub", matchers.Pub)
	xls = newMIME("application/vnd.ms-excel", ".xls", matchers.Xls).
		alias("application/msexcel")
	ps   = newMIME("application/postscript", ".ps", matchers.Ps)
	fits = newMIME("application/fits", ".fits", matchers.Fits)
	ogg  = newMIME("application/ogg", ".ogg", matchers.Ogg, oggAudio, oggVideo).
		alias("application/x-ogg")
	oggAudio = newMIME("audio/ogg", ".oga", matchers.OggAudio)
	oggVideo = newMIME("video/ogg", ".ogv", matchers.OggVideo)
	utf32le  = newMIME("text/plain; charset=utf-32le", ".txt", matchers.Utf32le)
	utf32be  = newMIME("text/plain; charset=utf-32be", ".txt", matchers.Utf32be)
	utf16le  = newMIME("text/plain; charset=utf-16le", ".txt", matchers.Utf16le)
	utf16be  = newMIME("text/plain; charset=utf-16be", ".txt", matchers.Utf16be)
	utf8     = newMIME("text/plain; charset=utf-8", ".txt", matchers.Utf8, html, svg, xml, php, js, lua, perl, python, json, ndJson, rtf, tcl, csv, tsv, vCard, iCalendar, warc)
	xml      = newMIME("text/xml; charset=utf-8", ".xml", matchers.Xml, rss, atom, x3d, kml, xliff, collada, gml, gpx, tcx, amf, threemf)
	json     = newMIME("application/json", ".json", matchers.Json, geoJson)
	csv      = newMIME("text/csv", ".csv", matchers.Csv)
	tsv      = newMIME("text/tab-separated-values", ".tsv", matchers.Tsv)
	geoJson  = newMIME("application/geo+json", ".geojson", matchers.GeoJson)
	ndJson   = newMIME("application/x-ndjson", ".ndjson", matchers.NdJson)
	html     = newMIME("text/html; charset=utf-8", ".html", matchers.Html)
	php      = newMIME("text/x-php; charset=utf-8", ".php", matchers.Php)
	rtf      = newMIME("text/rtf", ".rtf", matchers.Rtf)
	js       = newMIME("application/javascript", ".js", matchers.Js).
			alias("application/x-javascript", "text/javascript")
	lua    = newMIME("text/x-lua", ".lua", matchers.Lua)
	perl   = newMIME("text/x-perl", ".pl", matchers.Perl)
	python = newMIME("application/x-python", ".py", matchers.Python)
	tcl    = newMIME("text/x-tcl", ".tcl", matchers.Tcl).
		alias("application/x-tcl")
	vCard     = newMIME("text/vcard", ".vcf", matchers.VCard)
	iCalendar = newMIME("text/calendar", ".ics", matchers.ICalendar)
	svg       = newMIME("image/svg+xml", ".svg", matchers.Svg)
	rss       = newMIME("application/rss+xml", ".rss", matchers.Rss).
			alias("text/rss")
	atom    = newMIME("application/atom+xml", ".atom", matchers.Atom)
	x3d     = newMIME("model/x3d+xml", ".x3d", matchers.X3d)
	kml     = newMIME("application/vnd.google-earth.kml+xml", ".kml", matchers.Kml)
	xliff   = newMIME("application/x-xliff+xml", ".xlf", matchers.Xliff)
	collada = newMIME("model/vnd.collada+xml", ".dae", matchers.Collada)
	gml     = newMIME("application/gml+xml", ".gml", matchers.Gml)
	gpx     = newMIME("application/gpx+xml", ".gpx", matchers.Gpx)
	tcx     = newMIME("application/vnd.garmin.tcx+xml", ".tcx", matchers.Tcx)
	amf     = newMIME("application/x-amf", ".amf", matchers.Amf)
	threemf = newMIME("application/vnd.ms-package.3dmanufacturing-3dmodel+xml", ".3mf", matchers.Threemf)
	png     = newMIME("image/png", ".png", matchers.Png)
	jpg     = newMIME("image/jpeg", ".jpg", matchers.Jpg)
	jp2     = newMIME("image/jp2", ".jp2", matchers.Jp2)
	jpx     = newMIME("image/jpx", ".jpf", matchers.Jpx)
	jpm     = newMIME("image/jpm", ".jpm", matchers.Jpm).
		alias("video/jpm")
	bpg  = newMIME("image/bpg", ".bpg", matchers.Bpg)
	gif  = newMIME("image/gif", ".gif", matchers.Gif)
	webp = newMIME("image/webp", ".webp", matchers.Webp)
	tiff = newMIME("image/tiff", ".tiff", matchers.Tiff)
	bmp  = newMIME("image/bmp", ".bmp", matchers.Bmp).
		alias("image/x-bmp", "image/x-ms-bmp")
	ico  = newMIME("image/x-icon", ".ico", matchers.Ico)
	icns = newMIME("image/x-icns", ".icns", matchers.Icns)
	psd  = newMIME("image/vnd.adobe.photoshop", ".psd", matchers.Psd).
		alias("image/x-psd", "application/photoshop")
	heic    = newMIME("image/heic", ".heic", matchers.Heic)
	heicSeq = newMIME("image/heic-sequence", ".heic", matchers.HeicSequence)
	heif    = newMIME("image/heif", ".heif", matchers.Heif)
	heifSeq = newMIME("image/heif-sequence", ".heif", matchers.HeifSequence)
	mp3     = newMIME("audio/mpeg", ".mp3", matchers.Mp3).
		alias("audio/x-mpeg", "audio/mp3")
	flac = newMIME("audio/flac", ".flac", matchers.Flac)
	midi = newMIME("audio/midi", ".midi", matchers.Midi).
		alias("audio/mid", "audio/sp-midi", "audio/x-mid", "audio/x-midi")
	ape      = newMIME("audio/ape", ".ape", matchers.Ape)
	musePack = newMIME("audio/musepack", ".mpc", matchers.MusePack)
	wav      = newMIME("audio/wav", ".wav", matchers.Wav).
			alias("audio/x-wav", "audio/vnd.wave", "audio/wave")
	aiff = newMIME("audio/aiff", ".aiff", matchers.Aiff)
	au   = newMIME("audio/basic", ".au", matchers.Au)
	amr  = newMIME("audio/amr", ".amr", matchers.Amr).
		alias("audio/amr-nb")
	aac  = newMIME("audio/aac", ".aac", matchers.Aac)
	voc  = newMIME("audio/x-unknown", ".voc", matchers.Voc)
	aMp4 = newMIME("audio/mp4", ".mp4", matchers.AMp4).
		alias("audio/x-m4a", "audio/x-mp4a")
	m4a  = newMIME("audio/x-m4a", ".m4a", matchers.M4a)
	mp4  = newMIME("video/mp4", ".mp4", matchers.Mp4)
	webM = newMIME("video/webm", ".webm", matchers.WebM).
		alias("audio/webm")
	mpeg      = newMIME("video/mpeg", ".mpeg", matchers.Mpeg)
	quickTime = newMIME("video/quicktime", ".mov", matchers.QuickTime)
	mqv       = newMIME("video/quicktime", ".mqv", matchers.Mqv)
	threeGP   = newMIME("video/3gpp", ".3gp", matchers.ThreeGP).
			alias("video/3gp", "audio/3gpp")
	threeG2 = newMIME("video/3gpp2", ".3g2", matchers.ThreeG2).
		alias("video/3g2", "audio/3gpp2")
	avi = newMIME("video/x-msvideo", ".avi", matchers.Avi).
		alias("video/avi", "video/msvideo")
	flv = newMIME("video/x-flv", ".flv", matchers.Flv)
	mkv = newMIME("video/x-matroska", ".mkv", matchers.Mkv)
	asf = newMIME("video/x-ms-asf", ".asf", matchers.Asf).
		alias("video/asf", "video/x-ms-wmv")
	class   = newMIME("application/x-java-applet; charset=binary", ".class", matchers.Class)
	swf     = newMIME("application/x-shockwave-flash", ".swf", matchers.Swf)
	crx     = newMIME("application/x-chrome-extension", ".crx", matchers.Crx)
	woff    = newMIME("font/woff", ".woff", matchers.Woff)
	woff2   = newMIME("font/woff2", ".woff2", matchers.Woff2)
	otf     = newMIME("font/otf", ".otf", matchers.Otf)
	eot     = newMIME("application/vnd.ms-fontobject", ".eot", matchers.Eot)
	wasm    = newMIME("application/wasm", ".wasm", matchers.Wasm)
	shp     = newMIME("application/octet-stream", ".shp", matchers.Shp)
	shx     = newMIME("application/octet-stream", ".shx", matchers.Shx, shp)
	dbf     = newMIME("application/x-dbf", ".dbf", matchers.Dbf)
	exe     = newMIME("application/vnd.microsoft.portable-executable", ".exe", matchers.Exe)
	elf     = newMIME("application/x-elf", "", matchers.Elf, elfObj, elfExe, elfLib, elfDump)
	elfObj  = newMIME("application/x-object", "", matchers.ElfObj)
	elfExe  = newMIME("application/x-executable", "", matchers.ElfExe)
	elfLib  = newMIME("application/x-sharedlib", ".so", matchers.ElfLib)
	elfDump = newMIME("application/x-coredump", "", matchers.ElfDump)
	ar      = newMIME("application/x-archive", ".a", matchers.Ar, deb).
		alias("application/x-unix-archive")
	deb = newMIME("application/vnd.debian.binary-package", ".deb", matchers.Deb)
	dcm = newMIME("application/dicom", ".dcm", matchers.Dcm)
	odt = newMIME("application/vnd.oasis.opendocument.text", ".odt", matchers.Odt, ott).
		alias("application/x-vnd.oasis.opendocument.text")
	ott = newMIME("application/vnd.oasis.opendocument.text-template", ".ott", matchers.Ott).
		alias("application/x-vnd.oasis.opendocument.text-template")
	ods = newMIME("application/vnd.oasis.opendocument.spreadsheet", ".ods", matchers.Ods, ots).
		alias("application/x-vnd.oasis.opendocument.spreadsheet")
	ots = newMIME("application/vnd.oasis.opendocument.spreadsheet-template", ".ots", matchers.Ots).
		alias("application/x-vnd.oasis.opendocument.spreadsheet-template")
	odp = newMIME("application/vnd.oasis.opendocument.presentation", ".odp", matchers.Odp, otp).
		alias("application/x-vnd.oasis.opendocument.presentation")
	otp = newMIME("application/vnd.oasis.opendocument.presentation-template", ".otp", matchers.Otp).
		alias("application/x-vnd.oasis.opendocument.presentation-template")
	odg = newMIME("application/vnd.oasis.opendocument.graphics", ".odg", matchers.Odg, otg).
		alias("application/x-vnd.oasis.opendocument.graphics")
	otg = newMIME("application/vnd.oasis.opendocument.graphics-template", ".otg", matchers.Otg).
		alias("application/x-vnd.oasis.opendocument.graphics-template")
	odf = newMIME("application/vnd.oasis.opendocument.formula", ".odf", matchers.Odf).
		alias("application/x-vnd.oasis.opendocument.formula")
	rar = newMIME("application/x-rar-compressed", ".rar", matchers.Rar).
		alias("application/x-rar")
	djvu    = newMIME("image/vnd.djvu", ".djvu", matchers.DjVu)
	mobi    = newMIME("application/x-mobipocket-ebook", ".mobi", matchers.Mobi)
	lit     = newMIME("application/x-ms-reader", ".lit", matchers.Lit)
	sqlite3 = newMIME("application/x-sqlite3", ".sqlite", matchers.Sqlite)
	dwg     = newMIME("image/vnd.dwg", ".dwg", matchers.Dwg).
		alias("image/x-dwg", "application/acad", "application/x-acad", "application/autocad_dwg", "application/dwg", "application/x-dwg", "application/x-autocad", "drawing/dwg")
	warc  = newMIME("application/warc", ".warc", matchers.Warc)
	nes   = newMIME("application/vnd.nintendo.snes.rom", ".nes", matchers.Nes)
	macho = newMIME("application/x-mach-binary", ".macho", matchers.MachO)
	qcp   = newMIME("audio/qcelp", ".qcp", matchers.Qcp)
	mrc   = newMIME("application/marc", ".mrc", matchers.Marc)
	mdb   = newMIME("application/x-msaccess", ".mdb", matchers.MsAccessMdb)
	accdb = newMIME("application/x-msaccess", ".accdb", matchers.MsAccessAce)
	zstd  = newMIME("application/zstd", ".zst", matchers.Zstd)
	cab   = newMIME("application/vnd.ms-cab-compressed", ".cab", matchers.Cab)
)
