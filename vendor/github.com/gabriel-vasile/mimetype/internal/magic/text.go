package magic

import (
	"bufio"
	"bytes"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype/internal/charset"
	"github.com/gabriel-vasile/mimetype/internal/json"
)

var (
	// HTML matches a Hypertext Markup Language file.
	HTML = markup(
		[]byte("<!DOCTYPE HTML"),
		[]byte("<HTML"),
		[]byte("<HEAD"),
		[]byte("<SCRIPT"),
		[]byte("<IFRAME"),
		[]byte("<H1"),
		[]byte("<DIV"),
		[]byte("<FONT"),
		[]byte("<TABLE"),
		[]byte("<A"),
		[]byte("<STYLE"),
		[]byte("<TITLE"),
		[]byte("<B"),
		[]byte("<BODY"),
		[]byte("<BR"),
		[]byte("<P"),
	)
	// XML matches an Extensible Markup Language file.
	XML = markup([]byte("<?XML"))
	// Owl2 matches an Owl ontology file.
	Owl2 = xml(newXMLSig("Ontology", `xmlns="http://www.w3.org/2002/07/owl#"`))
	// Rss matches a Rich Site Summary file.
	Rss = xml(newXMLSig("rss", ""))
	// Atom matches an Atom Syndication Format file.
	Atom = xml(newXMLSig("feed", `xmlns="http://www.w3.org/2005/Atom"`))
	// Kml matches a Keyhole Markup Language file.
	Kml = xml(
		newXMLSig("kml", `xmlns="http://www.opengis.net/kml/2.2"`),
		newXMLSig("kml", `xmlns="http://earth.google.com/kml/2.0"`),
		newXMLSig("kml", `xmlns="http://earth.google.com/kml/2.1"`),
		newXMLSig("kml", `xmlns="http://earth.google.com/kml/2.2"`),
	)
	// Xliff matches a XML Localization Interchange File Format file.
	Xliff = xml(newXMLSig("xliff", `xmlns="urn:oasis:names:tc:xliff:document:1.2"`))
	// Collada matches a COLLAborative Design Activity file.
	Collada = xml(newXMLSig("COLLADA", `xmlns="http://www.collada.org/2005/11/COLLADASchema"`))
	// Gml matches a Geography Markup Language file.
	Gml = xml(
		newXMLSig("", `xmlns:gml="http://www.opengis.net/gml"`),
		newXMLSig("", `xmlns:gml="http://www.opengis.net/gml/3.2"`),
		newXMLSig("", `xmlns:gml="http://www.opengis.net/gml/3.3/exr"`),
	)
	// Gpx matches a GPS Exchange Format file.
	Gpx = xml(newXMLSig("gpx", `xmlns="http://www.topografix.com/GPX/1/1"`))
	// Tcx matches a Training Center XML file.
	Tcx = xml(newXMLSig("TrainingCenterDatabase", `xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2"`))
	// X3d matches an Extensible 3D Graphics file.
	X3d = xml(newXMLSig("X3D", `xmlns:xsd="http://www.w3.org/2001/XMLSchema-instance"`))
	// Amf matches an Additive Manufacturing XML file.
	Amf = xml(newXMLSig("amf", ""))
	// Threemf matches a 3D Manufacturing Format file.
	Threemf = xml(newXMLSig("model", `xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02"`))
	// Xfdf matches a XML Forms Data Format file.
	Xfdf = xml(newXMLSig("xfdf", `xmlns="http://ns.adobe.com/xfdf/"`))
	// VCard matches a Virtual Contact File.
	VCard = ciPrefix([]byte("BEGIN:VCARD\n"), []byte("BEGIN:VCARD\r\n"))
	// ICalendar matches a iCalendar file.
	ICalendar = ciPrefix([]byte("BEGIN:VCALENDAR\n"), []byte("BEGIN:VCALENDAR\r\n"))
	phpPageF  = ciPrefix(
		[]byte("<?PHP"),
		[]byte("<?\n"),
		[]byte("<?\r"),
		[]byte("<? "),
	)
	phpScriptF = shebang(
		[]byte("/usr/local/bin/php"),
		[]byte("/usr/bin/php"),
		[]byte("/usr/bin/env php"),
	)
	// Js matches a Javascript file.
	Js = shebang(
		[]byte("/bin/node"),
		[]byte("/usr/bin/node"),
		[]byte("/bin/nodejs"),
		[]byte("/usr/bin/nodejs"),
		[]byte("/usr/bin/env node"),
		[]byte("/usr/bin/env nodejs"),
	)
	// Lua matches a Lua programming language file.
	Lua = shebang(
		[]byte("/usr/bin/lua"),
		[]byte("/usr/local/bin/lua"),
		[]byte("/usr/bin/env lua"),
	)
	// Perl matches a Perl programming language file.
	Perl = shebang(
		[]byte("/usr/bin/perl"),
		[]byte("/usr/bin/env perl"),
	)
	// Python matches a Python programming language file.
	Python = shebang(
		[]byte("/usr/bin/python"),
		[]byte("/usr/local/bin/python"),
		[]byte("/usr/bin/env python"),
	)
	// Tcl matches a Tcl programming language file.
	Tcl = shebang(
		[]byte("/usr/bin/tcl"),
		[]byte("/usr/local/bin/tcl"),
		[]byte("/usr/bin/env tcl"),
		[]byte("/usr/bin/tclsh"),
		[]byte("/usr/local/bin/tclsh"),
		[]byte("/usr/bin/env tclsh"),
		[]byte("/usr/bin/wish"),
		[]byte("/usr/local/bin/wish"),
		[]byte("/usr/bin/env wish"),
	)
	// Rtf matches a Rich Text Format file.
	Rtf = prefix([]byte("{\\rtf1"))
)

// Text matches a plain text file.
//
// TODO: This function does not parse BOM-less UTF16 and UTF32 files. Not really
// sure it should. Linux file utility also requires a BOM for UTF16 and UTF32.
func Text(raw []byte, limit uint32) bool {
	// First look for BOM.
	if cset := charset.FromBOM(raw); cset != "" {
		return true
	}
	// Binary data bytes as defined here: https://mimesniff.spec.whatwg.org/#binary-data-byte
	for _, b := range raw {
		if b <= 0x08 ||
			b == 0x0B ||
			0x0E <= b && b <= 0x1A ||
			0x1C <= b && b <= 0x1F {
			return false
		}
	}
	return true
}

// Php matches a PHP: Hypertext Preprocessor file.
func Php(raw []byte, limit uint32) bool {
	if res := phpPageF(raw, limit); res {
		return res
	}
	return phpScriptF(raw, limit)
}

// JSON matches a JavaScript Object Notation file.
func JSON(raw []byte, limit uint32) bool {
	raw = trimLWS(raw)
	// #175 A single JSON string, number or bool is not considered JSON.
	// JSON objects and arrays are reported as JSON.
	if len(raw) < 2 || (raw[0] != '[' && raw[0] != '{') {
		return false
	}
	parsed, err := json.Scan(raw)
	// If the full file content was provided, check there is no error.
	if limit == 0 || len(raw) < int(limit) {
		return err == nil
	}

	// If a section of the file was provided, check if all of it was parsed.
	return parsed == len(raw) && len(raw) > 0
}

// GeoJSON matches a RFC 7946 GeoJSON file.
//
// GeoJSON detection implies searching for key:value pairs like: `"type": "Feature"`
// in the input.
// BUG(gabriel-vasile): The "type" key should be searched for in the root object.
func GeoJSON(raw []byte, limit uint32) bool {
	raw = trimLWS(raw)
	if len(raw) == 0 {
		return false
	}
	// GeoJSON is always a JSON object, not a JSON array or any other JSON value.
	if raw[0] != '{' {
		return false
	}

	s := []byte(`"type"`)
	si, sl := bytes.Index(raw, s), len(s)

	if si == -1 {
		return false
	}

	// If the "type" string is the suffix of the input,
	// there is no need to search for the value of the key.
	if si+sl == len(raw) {
		return false
	}
	// Skip the "type" part.
	raw = raw[si+sl:]
	// Skip any whitespace before the colon.
	raw = trimLWS(raw)
	// Check for colon.
	if len(raw) == 0 || raw[0] != ':' {
		return false
	}
	// Skip any whitespace after the colon.
	raw = trimLWS(raw[1:])

	geoJSONTypes := [][]byte{
		[]byte(`"Feature"`),
		[]byte(`"FeatureCollection"`),
		[]byte(`"Point"`),
		[]byte(`"LineString"`),
		[]byte(`"Polygon"`),
		[]byte(`"MultiPoint"`),
		[]byte(`"MultiLineString"`),
		[]byte(`"MultiPolygon"`),
		[]byte(`"GeometryCollection"`),
	}
	for _, t := range geoJSONTypes {
		if bytes.HasPrefix(raw, t) {
			return true
		}
	}

	return false
}

// NdJSON matches a Newline delimited JSON file. All complete lines from raw
// must be valid JSON documents meaning they contain one of the valid JSON data
// types.
func NdJSON(raw []byte, limit uint32) bool {
	lCount, hasObjOrArr := 0, false
	sc := bufio.NewScanner(dropLastLine(raw, limit))
	for sc.Scan() {
		l := sc.Bytes()
		// Empty lines are allowed in NDJSON.
		if l = trimRWS(trimLWS(l)); len(l) == 0 {
			continue
		}
		_, err := json.Scan(l)
		if err != nil {
			return false
		}
		if l[0] == '[' || l[0] == '{' {
			hasObjOrArr = true
		}
		lCount++
	}

	return lCount > 1 && hasObjOrArr
}

// HAR matches a HAR Spec file.
// Spec: http://www.softwareishard.com/blog/har-12-spec/
func HAR(raw []byte, limit uint32) bool {
	s := []byte(`"log"`)
	si, sl := bytes.Index(raw, s), len(s)

	if si == -1 {
		return false
	}

	// If the "log" string is the suffix of the input,
	// there is no need to search for the value of the key.
	if si+sl == len(raw) {
		return false
	}
	// Skip the "log" part.
	raw = raw[si+sl:]
	// Skip any whitespace before the colon.
	raw = trimLWS(raw)
	// Check for colon.
	if len(raw) == 0 || raw[0] != ':' {
		return false
	}
	// Skip any whitespace after the colon.
	raw = trimLWS(raw[1:])

	harJSONTypes := [][]byte{
		[]byte(`"version"`),
		[]byte(`"creator"`),
		[]byte(`"entries"`),
	}
	for _, t := range harJSONTypes {
		si := bytes.Index(raw, t)
		if si > -1 {
			return true
		}
	}

	return false
}

// Svg matches a SVG file.
func Svg(raw []byte, limit uint32) bool {
	return bytes.Contains(raw, []byte("<svg"))
}

// Srt matches a SubRip file.
func Srt(in []byte, _ uint32) bool {
	s := bufio.NewScanner(bytes.NewReader(in))
	if !s.Scan() {
		return false
	}
	// First line must be 1.
	if s.Text() != "1" {
		return false
	}

	if !s.Scan() {
		return false
	}
	secondLine := s.Text()
	// Timestamp format (e.g: 00:02:16,612 --> 00:02:19,376) limits secondLine
	// length to exactly 29 characters.
	if len(secondLine) != 29 {
		return false
	}
	// Decimal separator of fractional seconds in the timestamps must be a
	// comma, not a period.
	if strings.Contains(secondLine, ".") {
		return false
	}
	// For Go <1.17, comma is not recognised as a decimal separator by `time.Parse`.
	secondLine = strings.ReplaceAll(secondLine, ",", ".")
	// Second line must be a time range.
	ts := strings.Split(secondLine, " --> ")
	if len(ts) != 2 {
		return false
	}
	const layout = "15:04:05.000"
	t0, err := time.Parse(layout, ts[0])
	if err != nil {
		return false
	}
	t1, err := time.Parse(layout, ts[1])
	if err != nil {
		return false
	}
	if t0.After(t1) {
		return false
	}

	// A third line must exist and not be empty. This is the actual subtitle text.
	return s.Scan() && len(s.Bytes()) != 0
}

// Vtt matches a Web Video Text Tracks (WebVTT) file. See
// https://www.iana.org/assignments/media-types/text/vtt.
func Vtt(raw []byte, limit uint32) bool {
	// Prefix match.
	prefixes := [][]byte{
		{0xEF, 0xBB, 0xBF, 0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x0A}, // UTF-8 BOM, "WEBVTT" and a line feed
		{0xEF, 0xBB, 0xBF, 0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x0D}, // UTF-8 BOM, "WEBVTT" and a carriage return
		{0xEF, 0xBB, 0xBF, 0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x20}, // UTF-8 BOM, "WEBVTT" and a space
		{0xEF, 0xBB, 0xBF, 0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x09}, // UTF-8 BOM, "WEBVTT" and a horizontal tab
		{0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x0A},                   // "WEBVTT" and a line feed
		{0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x0D},                   // "WEBVTT" and a carriage return
		{0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x20},                   // "WEBVTT" and a space
		{0x57, 0x45, 0x42, 0x56, 0x54, 0x54, 0x09},                   // "WEBVTT" and a horizontal tab
	}
	for _, p := range prefixes {
		if bytes.HasPrefix(raw, p) {
			return true
		}
	}

	// Exact match.
	return bytes.Equal(raw, []byte{0xEF, 0xBB, 0xBF, 0x57, 0x45, 0x42, 0x56, 0x54, 0x54}) || // UTF-8 BOM and "WEBVTT"
		bytes.Equal(raw, []byte{0x57, 0x45, 0x42, 0x56, 0x54, 0x54}) // "WEBVTT"
}
