package matchers

import (
	"bytes"

	"github.com/gabriel-vasile/mimetype/internal/json"
)

var (
	htmlSigs = []sig{
		markupSig("<!DOCTYPE HTML"),
		markupSig("<HTML"),
		markupSig("<HEAD"),
		markupSig("<SCRIPT"),
		markupSig("<IFRAME"),
		markupSig("<H1"),
		markupSig("<DIV"),
		markupSig("<FONT"),
		markupSig("<TABLE"),
		markupSig("<A"),
		markupSig("<STYLE"),
		markupSig("<TITLE"),
		markupSig("<B"),
		markupSig("<BODY"),
		markupSig("<BR"),
		markupSig("<P"),
		markupSig("<!--"),
	}
	xmlSigs = []sig{
		markupSig("<?XML"),
	}
	kmlSigs = []sig{
		newXmlSig("kml", `xmlns="http://www.opengis.net/kml/2.2"`),
		newXmlSig("kml", `xmlns="http://earth.google.com/kml/2.0"`),
		newXmlSig("kml", `xmlns="http://earth.google.com/kml/2.1"`),
		newXmlSig("kml", `xmlns="http://earth.google.com/kml/2.2"`),
	}
	colladaSigs = []sig{
		newXmlSig("COLLADA", `xmlns="http://www.collada.org/2005/11/COLLADASchema"`),
	}
	gmlSigs = []sig{
		newXmlSig("", `xmlns:gml="http://www.opengis.net/gml/3.2"`),
		newXmlSig("", `xmlns:gml="http://www.opengis.net/gml/3.3/exr"`),
	}
	gpxSigs = []sig{
		newXmlSig("gpx", `xmlns="http://www.topografix.com/GPX/1/1"`),
	}
	tcxSigs = []sig{
		newXmlSig("TrainingCenterDatabase", `xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2"`),
	}
	x3dSigs = []sig{
		newXmlSig("X3D", `xmlns:xsd="http://www.w3.org/2001/XMLSchema-instance"`),
	}
	vCardSigs = []sig{
		ciSig("BEGIN:VCARD\n"),
	}
	phpSigs = []sig{
		ciSig("<?PHP"),
		ciSig("<?\n"),
		ciSig("<?\r"),
		ciSig("<? "),
		shebangSig("/usr/local/bin/php"),
		shebangSig("/usr/bin/php"),
		shebangSig("/usr/bin/env php"),
	}
	jsSigs = []sig{
		shebangSig("/bin/node"),
		shebangSig("/usr/bin/node"),
		shebangSig("/bin/nodejs"),
		shebangSig("/usr/bin/nodejs"),
		shebangSig("/usr/bin/env node"),
		shebangSig("/usr/bin/env nodejs"),
	}
	luaSigs = []sig{
		shebangSig("/usr/bin/lua"),
		shebangSig("/usr/local/bin/lua"),
		shebangSig("/usr/bin/env lua"),
	}
	perlSigs = []sig{
		shebangSig("/usr/bin/perl"),
		shebangSig("/usr/bin/env perl"),
	}
	pythonSigs = []sig{
		shebangSig("/usr/bin/python"),
		shebangSig("/usr/local/bin/python"),
		shebangSig("/usr/bin/env python"),
	}
	tclSigs = []sig{
		shebangSig("/usr/bin/tcl"),
		shebangSig("/usr/local/bin/tcl"),
		shebangSig("/usr/bin/env tcl"),
		shebangSig("/usr/bin/tclsh"),
		shebangSig("/usr/local/bin/tclsh"),
		shebangSig("/usr/bin/env tclsh"),
		shebangSig("/usr/bin/wish"),
		shebangSig("/usr/local/bin/wish"),
		shebangSig("/usr/bin/env wish"),
	}
)

// Txt matches a text file.
func Txt(in []byte) bool {
	in = trimLWS(in)
	for _, b := range in {
		if b <= 0x08 ||
			b == 0x0B ||
			0x0E <= b && b <= 0x1A ||
			0x1C <= b && b <= 0x1F {
			return false
		}
	}

	return true
}

// Html matches a Hypertext Markup Language file.
func Html(in []byte) bool {
	return detect(in, htmlSigs)
}

// Xml matches an Extensible Markup Language file.
func Xml(in []byte) bool {
	return detect(in, xmlSigs)
}

// Php matches a PHP: Hypertext Preprocessor file.
func Php(in []byte) bool {
	return detect(in, phpSigs)
}

// Json matches a JavaScript Object Notation file.
func Json(in []byte) bool {
	parsed, err := json.Scan(in)
	if len(in) < ReadLimit {
		return err == nil
	}

	return parsed == len(in)
}

// GeoJson matches a RFC 7946 GeoJSON file.
//
// BUG(gabriel-vasile): The "type" key should be searched for in the root object.
func GeoJson(in []byte) bool {
	in = trimLWS(in)
	// geojson is always an object
	if in[0] != '{' {
		return false
	}

	s := []byte(`"type"`)
	si := bytes.Index(in, s)
	sl := len(s)

	if si == -1 {
		return false
	}

	// skip the "type" part
	in = in[si+sl:]
	// skip any whitespace before the colon
	in = trimLWS(in)
	// skip any whitesapce after the colon
	// not checking if char is colon because json matcher already did check
	in = trimLWS(in[1:])

	geoJsonTypes := [][]byte{
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
	for _, t := range geoJsonTypes {
		if bytes.HasPrefix(in, t) {
			return true
		}
	}

	return false
}

// Js matches a Javascript file.
func Js(in []byte) bool {
	return detect(in, jsSigs)
}

// Lua matches a Lua programming language file.
func Lua(in []byte) bool {
	return detect(in, luaSigs)
}

// Perl matches a Perl programming language file.
func Perl(in []byte) bool {
	return detect(in, perlSigs)
}

// Python matches a Python programming language file.
func Python(in []byte) bool {
	return detect(in, pythonSigs)
}

// Tcl matches a Tcl programming language file.
func Tcl(in []byte) bool {
	return detect(in, tclSigs)
}

// Rtf matches a Rich Text Format file.
func Rtf(in []byte) bool {
	return len(in) > 6 && bytes.Equal(in[:6], []byte("{\\rtf1"))
}

// Svg matches a SVG file.
func Svg(in []byte) bool {
	return bytes.Contains(in, []byte("<svg"))
}

// Kml matches a Keyhole Markup Language file.
func Kml(in []byte) bool {
	return detect(in, kmlSigs)
}

// Collada matches a COLLAborative Design Activity file.
func Collada(in []byte) bool {
	return detect(in, colladaSigs)
}

// Gml matches a Geography Markup Language file.
func Gml(in []byte) bool {
	return detect(in, gmlSigs)
}

// Gpx matches a GPS Exchange Format file.
func Gpx(in []byte) bool {
	return detect(in, gpxSigs)
}

// Tcx matches a Training Center XML file.
func Tcx(in []byte) bool {
	return detect(in, tcxSigs)
}

// X3d matches an Extensible 3D Graphics file.
func X3d(in []byte) bool {
	return detect(in, x3dSigs)
}

// VCard matches a Virtual Contact File.
func VCard(in []byte) bool {
	return detect(in, vCardSigs)
}
