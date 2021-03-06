package wofdata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomtaylor/wof-sync-os-postcodes/onsdb"
	"github.com/tomtaylor/wof-sync-os-postcodes/pipclient"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	exporter "github.com/whosonfirst/go-whosonfirst-export/exporter"
	"github.com/whosonfirst/go-whosonfirst-export/options"
	geojson "github.com/whosonfirst/go-whosonfirst-geojson-v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"
	index "github.com/whosonfirst/go-whosonfirst-index"
	uri "github.com/whosonfirst/go-whosonfirst-uri"

	// Enable the filesystem driver
	_ "github.com/whosonfirst/go-whosonfirst-index/fs"
)

type WOFData struct {
	dataPath string
	exp      exporter.Exporter
}

func NewWOFData(dataPath string, expOpts options.Options) *WOFData {
	exp, err := exporter.NewWhosOnFirstExporter(expOpts)
	if err != nil {
		return nil
	}

	data := &WOFData{dataPath: dataPath, exp: exp}

	return data
}

// Iterate fires the provided callback for every file in the WOFData path.
func (d *WOFData) Iterate(cb func(geojson.Feature) error) error {
	localCb := func(ctx context.Context, fh io.Reader, args ...interface{}) error {
		path, err := index.PathForContext(ctx)
		if err != nil {
			return err
		}

		f, err := feature.LoadWOFFeatureFromFile(path)
		if err != nil {
			return err
		}

		err = cb(f)

		return err
	}

	i, err := index.NewIndexer("directory", localCb)
	if err != nil {
		return err
	}

	err = i.IndexPath(d.dataPath)
	if err != nil {
		return err
	}

	return nil
}

const edtfDateLayout = "2006-01-02"

// DeprecateFeature deprecates the provided feature and writes it to disk.
func (d *WOFData) DeprecateFeature(f geojson.Feature, dryRun bool) (changed bool, err error) {
	bytes := f.Bytes()
	json := string(bytes)

	deprecated := "uuuu"
	result := gjson.Get(json, "properties.edtf:deprecated")
	if result.Exists() {
		deprecated = result.String()
	}

	if deprecated != "uuuu" {
		log.Printf("ID %s already deprecated, skipping", f.Id())
		return
	}

	now := time.Now()

	json, err = sjson.Set(json, "properties.edtf:deprecated", now.Format(edtfDateLayout))
	if err != nil {
		return
	}

	json, err = sjson.Set(json, "properties.mz:is_current", 0)
	if err != nil {
		return
	}

	changed = true

	if !dryRun {
		err = d.exportFeature(json)
	}

	return
}

// CeaseFeature ceases the provided feature and writes it to disk.
func (d *WOFData) CeaseFeature(f geojson.Feature, date time.Time, dryRun bool) (changed bool, err error) {
	bytes := f.Bytes()
	json := string(bytes)

	cessation := "uuuu"
	result := gjson.Get(json, "properties.edtf:cessation")
	if result.Exists() {
		cessation = result.String()
	}

	if cessation != "uuuu" {
		log.Printf("ID %s already ceased, skipping", f.Id())
		return
	}

	json, err = sjson.Set(json, "properties.edtf:cessation", date.Format(edtfDateLayout))
	if err != nil {
		return
	}

	json, err = sjson.Set(json, "properties.mz:is_current", 0)
	if err != nil {
		return
	}

	changed = true

	if !dryRun {
		err = d.exportFeature(json)
	}

	return
}

func (d *WOFData) UpdateFeature(f geojson.Feature, pcData *onsdb.PostcodeData, pip *pipclient.PIPClient, dryRun bool) (changed bool, err error) {
	bytes := f.Bytes()
	json := string(bytes)

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

	changed = true

	if !dryRun {
		err = d.exportFeature(json)
	}

	return
}

func (d *WOFData) NewFeature(pc *onsdb.PostcodeData, pip *pipclient.PIPClient) error {
	json := "{}"

	json, err := sjson.Set(json, "type", "Feature")
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:name", pc.Postcode)
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:placetype", "postalcode")
	if err != nil {
		return err
	}

	emptyList := make([]*string, 0)

	json, err = sjson.Set(json, "properties.wof:superseded_by", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:supersedes", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:breaches", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:tags", emptyList)
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:repo", "whosonfirst-data-postalcode-gb")
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.iso:country", "GB")
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.wof:country", "GB")
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.mz:hierarchy_label", 1)
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

	return d.exportFeature(json)
}

func (d *WOFData) exportFeature(json string) error {
	bytes := []byte(json)

	bytes, err := d.exp.Export(bytes)
	if err != nil {
		return err
	}

	idResult := gjson.GetBytes(bytes, "id")
	if !idResult.Exists() {
		return errors.New("Missing `id` field in JSON")
	}

	id := idResult.Int()

	path, err := uri.Id2AbsPath(d.dataPath, id)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	log.Printf("Writing to file %s", path)

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("Failed to close %s: %s", path, err)
		}
	}()

	_, err = f.Write(bytes)
	return err
}

func setDates(json string, pc *onsdb.PostcodeData) (string, error) {
	inception := convertStringToEDTF(pc.Inception)
	json, err := sjson.Set(json, "properties.edtf:inception", inception)
	if err != nil {
		return "", err
	}

	cessation := convertStringToEDTF(pc.Cessation)
	json, err = sjson.Set(json, "properties.edtf:cessation", cessation)
	if err != nil {
		return "", err
	}

	isCurrent := 1
	if cessation != "uuuu" {
		isCurrent = 0
	}

	json, err = sjson.Set(json, "properties.mz:is_current", isCurrent)
	if err != nil {
		return "", err
	}

	return json, nil
}

func setGeometry(json string, pc *onsdb.PostcodeData, pip *pipclient.PIPClient) (string, error) {
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
		json, err = sjson.Set(json, "properties.src:geom", "unknown")
		if err != nil {
			return json, err
		}

		json, err = sjson.Delete(json, "properties.wof:hierarchy")
		if err != nil {
			return json, err
		}

		return json, nil
	}

	// If we've updated the geometry, set the source to OS
	json, err = sjson.Set(json, "properties.src:geom", "os")
	if err != nil {
		return json, err
	}

	json, err = setHierarchy(json, pip, pc)
	if err != nil {
		return json, err
	}

	return json, nil
}

func setPointGeometry(json string, latitude string, longitude string) (string, error) {
	json, err := sjson.Set(json, "geometry.type", "Point")
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "geometry.coordinates.0", longitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "geometry.coordinates.1", latitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "bbox.0", longitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "bbox.1", latitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "bbox.2", longitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.SetRaw(json, "bbox.3", latitude)
	if err != nil {
		return "", err
	}

	return json, nil
}

func setOSProperties(json string, pc *onsdb.PostcodeData) (string, error) {
	// Drop the NHS fields, they're not very useful and we don't have NHS geography
	// anywhere else in WOF
	json, err := sjson.Delete(json, "properties.os:nhs_ha_code")
	if err != nil {
		return "", err
	}

	json, err = sjson.Delete(json, "properties.os:nhs_regional_ha_code")
	if err != nil {
		return "", err
	}

	// Delete this key because 'distict' is mispelt
	json, err = sjson.Delete(json, "properties.os:admin_distict_code")
	if err != nil {
		return "", err
	}

	// Drop the ward code, because these are part of electoral geography, which
	// isn't referenced anywhere else in WOF.
	json, err = sjson.Delete(json, "properties.os:admin_ward_code")
	if err != nil {
		return "", err
	}

	// Drop os:admin_county_code, because we're going to rename it later on.
	json, err = sjson.Delete(json, "properties.os:admin_county_code")
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.os:country_code", pc.CountryCode)
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.os:region_code", pc.RegionCode)
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.os:district_code", pc.DistrictCode)
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.os:county_code", pc.CountyCode)
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.os:positional_quality_indicator", pc.PositionalQuality)
	if err != nil {
		return "", err
	}

	return json, nil
}

func convertStringToEDTF(s string) string {
	if s == "" {
		return "uuuu"
	}

	t, err := time.Parse("200601", s)
	if err != nil {
		log.Fatalf("Failed to parse inception/cessation date %s: %s", s, err)
	}

	return t.Format(edtfDateLayout)
}

func setHierarchy(json string, pip *pipclient.PIPClient, pc *onsdb.PostcodeData) (string, error) {
	hierarchy, err := buildHierarchy(pip, pc.Latitude, pc.Longitude)
	if err != nil {
		return "", err
	}

	json, err = sjson.Set(json, "properties.wof:hierarchy.0", hierarchy)
	if err != nil {
		return "", err
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
