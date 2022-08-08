package wofdata

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sfomuseum/go-edtf"
	"github.com/whosonfirst/wof-sync-os-postcodes/onsdb"
	"github.com/whosonfirst/wof-sync-os-postcodes/pipclient"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/saracen/walker"
	export "github.com/whosonfirst/go-whosonfirst-export"
	"github.com/whosonfirst/go-whosonfirst-export/options"
	uri "github.com/whosonfirst/go-whosonfirst-uri"
)

type WOFData struct {
	dataPath      string
	exportOptions options.Options
}

func NewWOFData(dataPath string, expOpts options.Options) *WOFData {
	data := &WOFData{dataPath: dataPath, exportOptions: expOpts}

	return data
}

// Iterate fires the provided callback for every file in the WOFData path.
func (d *WOFData) Iterate(cb func([]byte) error) error {
	walkFn := func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".geojson") {
			return nil
		}

		f, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return cb(f)
	}

	errorFn := walker.WithErrorCallback(func(path string, err error) error {
		return err
	})

	return walker.Walk(d.dataPath, walkFn, errorFn)
}

const edtfDateLayout = "2006-01-02"

// DeprecateFeature deprecates the provided feature and writes it to disk.
func (d *WOFData) DeprecateFeature(f []byte, dryRun bool) (changed bool, err error) {
	originalBytes := make([]byte, len(f))
	copy(originalBytes, f)

	deprecated := edtf.UNSPECIFIED
	result := gjson.GetBytes(f, "properties.edtf:deprecated")
	if result.Exists() {
		deprecated = result.String()
	}

	idResult := gjson.GetBytes(f, "properties.id")
	var id int64 = -1
	if idResult.Exists() {
		id = idResult.Int()
	}

	if deprecated != edtf.UNSPECIFIED {
		log.Printf("ID %d already deprecated, skipping", id)
		return
	}

	now := time.Now()

	f, err = sjson.SetBytes(f, "properties.edtf:deprecated", now.Format(edtfDateLayout))
	if err != nil {
		return
	}

	f, err = sjson.SetBytes(f, "properties.mz:is_current", 0)
	if err != nil {
		return
	}

	if !dryRun {
		changed, err = d.exportFeature(f, originalBytes)
	}

	return
}

// CeaseFeature ceases the provided feature and writes it to disk.
func (d *WOFData) CeaseFeature(json []byte, date time.Time, dryRun bool) (changed bool, err error) {
	originalJSON := make([]byte, len(json))
	copy(originalJSON, json)

	cessation := edtf.UNSPECIFIED
	result := gjson.GetBytes(json, "properties.edtf:cessation")
	if result.Exists() {
		cessation = result.String()
	}

	idResult := gjson.GetBytes(json, "properties.id")
	var id int64 = -1
	if idResult.Exists() {
		id = idResult.Int()
	}

	if cessation != edtf.UNSPECIFIED {
		log.Printf("ID %d already ceased, skipping", id)
		return
	}

	json, err = sjson.SetBytes(json, "properties.edtf:cessation", date.Format(edtfDateLayout))
	if err != nil {
		return
	}

	json, err = sjson.SetBytes(json, "properties.mz:is_current", 0)
	if err != nil {
		return
	}

	if !dryRun {
		changed, err = d.exportFeature(json, originalJSON)
	}

	return
}

func (d *WOFData) UpdateFeature(json []byte, pcData *onsdb.PostcodeData, pip *pipclient.PIPClient, dryRun bool) (changed bool, err error) {
	originalJSON := make([]byte, len(json))
	copy(originalJSON, json)

	json, err = setDates(json, pcData)
	if err != nil {
		return
	}

	json, err = setGeometry(json, pcData, pip)
	if err != nil {
		return
	}

	json, err = setOSProperties(json, pcData)
	if err != nil {
		return
	}

	if !dryRun {
		changed, err = d.exportFeature(json, originalJSON)
	}

	return
}

func (d *WOFData) NewFeature(pc *onsdb.PostcodeData, pip *pipclient.PIPClient) error {
	json := []byte("{}")

	json, err := sjson.SetBytes(json, "type", "Feature")
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:name", pc.Postcode)
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:placetype", "postalcode")
	if err != nil {
		return err
	}

	emptyList := make([]*string, 0)

	json, err = sjson.SetBytes(json, "properties.wof:superseded_by", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:supersedes", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:breaches", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:tags", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:repo", "whosonfirst-data-postalcode-gb")
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.iso:country", "GB")
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.wof:country", "GB")
	if err != nil {
		return err
	}

	json, err = sjson.SetBytes(json, "properties.mz:hierarchy_label", 1)
	if err != nil {
		return err
	}

	json, err = setDates(json, pc)
	if err != nil {
		return err
	}

	json, err = setOSProperties(json, pc)
	if err != nil {
		return err
	}

	json, err = setGeometry(json, pc, pip)
	if err != nil {
		return err
	}

	_, err = d.exportFeature(json, []byte{})
	return err
}

func (d *WOFData) exportFeature(updatedBytes []byte, originalBytes []byte) (changed bool, err error) {
	var outputBuf bytes.Buffer
	writer := bufio.NewWriter(&outputBuf)

	changed, err = export.ExportChanged(updatedBytes, originalBytes, d.exportOptions, writer)
	if err != nil {
		return
	}

	if !changed {
		return
	}

	err = writer.Flush()
	if err != nil {
		return
	}

	idResult := gjson.GetBytes(updatedBytes, "id")
	if !idResult.Exists() {
		err = errors.New("missing `id` field in JSON")
		return
	}

	id := idResult.Int()

	path, err := uri.Id2AbsPath(d.dataPath, id)
	if err != nil {
		return
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}

	log.Printf("Writing to file %s", path)

	f, err := os.Create(path)
	if err != nil {
		return
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("Failed to close %s: %s", path, err)
		}
	}()

	_, err = f.Write(updatedBytes)
	return
}

func setDates(json []byte, pc *onsdb.PostcodeData) ([]byte, error) {
	inception := convertStringToEDTF(pc.Inception)
	json, err := sjson.SetBytes(json, "properties.edtf:inception", inception)
	if err != nil {
		return json, err
	}

	cessation := convertStringToEDTF(pc.Cessation)
	json, err = sjson.SetBytes(json, "properties.edtf:cessation", cessation)
	if err != nil {
		return json, err
	}

	isCurrent := 1
	if cessation != edtf.UNSPECIFIED {
		isCurrent = 0
	}

	json, err = sjson.SetBytes(json, "properties.mz:is_current", isCurrent)
	if err != nil {
		return json, err
	}

	return json, nil
}

func setGeometry(json []byte, pc *onsdb.PostcodeData, pip *pipclient.PIPClient) ([]byte, error) {
	latitude := pc.Latitude
	longitude := pc.Longitude

	// Set postcodes where we're not allowed to know where they are to null island
	if !shouldSetGeometry(pc) {
		latitude = "0.0"
		longitude = "0.0"
	}

	// Postcodes without geometry in the ONSDB are set to 99.999999
	if latitude == "99.999999" {
		latitude = "0.0"
	}

	json, err := setPointGeometry(json, latitude, longitude)
	if err != nil {
		return json, err
	}

	// If we have invalid geometry
	if latitude == "0.0" && longitude == "0.0" {
		// If there's no geometry set the source to unknown and clear the hierarchy
		json, err = sjson.SetBytes(json, "properties.src:geom", "unknown")
		if err != nil {
			return json, err
		}

		json, err = sjson.DeleteBytes(json, "properties.wof:hierarchy")
		if err != nil {
			return json, err
		}

		return json, nil
	}

	// If we've updated the geometry, set the source to OS
	json, err = sjson.SetBytes(json, "properties.src:geom", "os")
	if err != nil {
		return json, err
	}

	if pip != nil {
		json, err = setHierarchy(json, pip, pc)
		if err != nil {
			return json, err
		}
	}

	return json, nil
}

func setPointGeometry(json []byte, latitude string, longitude string) ([]byte, error) {
	latitudeBytes := []byte(latitude)
	longitudeBytes := []byte(longitude)

	json, err := sjson.SetBytes(json, "geometry.type", "Point")
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "geometry.coordinates.0", longitudeBytes)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "geometry.coordinates.1", latitudeBytes)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "bbox.0", longitudeBytes)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "bbox.1", latitudeBytes)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "bbox.2", longitudeBytes)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetRawBytes(json, "bbox.3", latitudeBytes)
	if err != nil {
		return json, err
	}

	return json, nil
}

func setOSProperties(json []byte, pc *onsdb.PostcodeData) ([]byte, error) {
	// Drop the NHS fields, they're not very useful and we don't have NHS geography
	// anywhere else in WOF
	json, err := sjson.DeleteBytes(json, "properties.os:nhs_ha_code")
	if err != nil {
		return json, err
	}

	json, err = sjson.DeleteBytes(json, "properties.os:nhs_regional_ha_code")
	if err != nil {
		return json, err
	}

	// Delete this key because 'distict' is mispelt
	json, err = sjson.DeleteBytes(json, "properties.os:admin_distict_code")
	if err != nil {
		return json, err
	}

	// Drop the ward code, because these are part of electoral geography, which
	// isn't referenced anywhere else in WOF.
	json, err = sjson.DeleteBytes(json, "properties.os:admin_ward_code")
	if err != nil {
		return json, err
	}

	// Drop os:admin_county_code, because we're going to rename it later on.
	json, err = sjson.DeleteBytes(json, "properties.os:admin_county_code")
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.os:country_code", pc.CountryCode)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.os:region_code", pc.RegionCode)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.os:district_code", pc.DistrictCode)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.os:county_code", pc.CountyCode)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.os:positional_quality_indicator", pc.PositionalQuality)
	if err != nil {
		return json, err
	}

	return json, nil
}

func convertStringToEDTF(s string) string {
	if s == "" {
		return edtf.UNSPECIFIED
	}

	t, err := time.Parse("200601", s)
	if err != nil {
		log.Fatalf("Failed to parse inception/cessation date %s: %s", s, err)
	}

	return t.Format(edtfDateLayout)
}

func setHierarchy(json []byte, pip *pipclient.PIPClient, pc *onsdb.PostcodeData) ([]byte, error) {
	hierarchy, err := buildHierarchy(pip, pc.Latitude, pc.Longitude)
	if err != nil {
		return json, err
	}

	json, err = sjson.SetBytes(json, "properties.wof:hierarchy.0", hierarchy)
	if err != nil {
		return json, err
	}

	return json, nil
}

func buildHierarchy(pip *pipclient.PIPClient, latitude string, longitude string) (map[string]int64, error) {
	h := make(map[string]int64)

	places, err := pip.PointInPolygon(latitude, longitude)
	if err != nil {
		return h, err
	}

	for _, place := range places.Places {
		placetype := place.WOFPlacetype
		key := fmt.Sprintf("%s_id", placetype)
		value := place.WOFId
		h[key] = value
	}

	return h, nil
}

// Don't set geometry for BT postcodes (Northern Ireland), because the
// licensing for these is more restrictive. 🙄
func shouldSetGeometry(pc *onsdb.PostcodeData) bool {
	name := pc.Postcode
	return !strings.HasPrefix(name, "BT")
}
