package matchers

import (
	"bytes"
	"encoding/binary"
)

// Shp matches a shape format file.
// https://www.esri.com/library/whitepapers/pdfs/shapefile.pdf
func Shp(in []byte) bool {
	if len(in) < 112 {
		return false
	}
	shapeTypes := []int{
		0,  // Null shape
		1,  // Point
		3,  // Polyline
		5,  // Polygon
		8,  // MultiPoint
		11, // PointZ
		13, // PolylineZ
		15, // PolygonZ
		18, // MultiPointZ
		21, // PointM
		23, // PolylineM
		25, // PolygonM
		28, // MultiPointM
		31, // MultiPatch
	}

	for _, st := range shapeTypes {
		if st == int(binary.LittleEndian.Uint32(in[108:112])) {
			return true
		}
	}

	return false
}

// Shx matches a shape index format file.
// https://www.esri.com/library/whitepapers/pdfs/shapefile.pdf
func Shx(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x00, 0x00, 0x27, 0x0A})
}
