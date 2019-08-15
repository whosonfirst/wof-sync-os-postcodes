package wofdata

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/onsdb"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/pipclient"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	export "github.com/whosonfirst/go-whosonfirst-export"
	"github.com/whosonfirst/go-whosonfirst-export/options"
	geojson "github.com/whosonfirst/go-whosonfirst-geojson-v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"
	index "github.com/whosonfirst/go-whosonfirst-index"
	uri "github.com/whosonfirst/go-whosonfirst-uri"
)

type WOFData struct {
	dataPath string
}

func NewWOFData(dataPath string) *WOFData {
	data := &WOFData{dataPath: dataPath}

	return data
}

func (d *WOFData) Iterate(cb func(geojson.Feature) error) error {
	localCb := func(fh io.Reader, ctx context.Context, args ...interface{}) error {
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

func (d *WOFData) DeprecateFeature(f geojson.Feature) error {
	bytes := f.Bytes()
	json := string(bytes)

	deprecated := "uuuu"
	result := gjson.Get(json, "properties.edtf:deprecated")
	if result.Exists() {
		deprecated = result.String()
	}

	if deprecated != "uuuu" {
		log.Printf("ID %s already deprecated, skipping", f.Id())
		return nil
	}

	now := time.Now()

	json, err := sjson.Set(json, "properties.edtf:deprecated", now.Format(edtfDateLayout))
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.mz:is_current", 0)
	if err != nil {
		return err
	}

	return d.exportFeature(f.Id(), json)
}

func (d *WOFData) CeaseFeature(f geojson.Feature, date time.Time) error {
	bytes := f.Bytes()
	json := string(bytes)

	cessation := "uuuu"
	result := gjson.Get(json, "properties.edtf:cessation")
	if result.Exists() {
		cessation = result.String()
	}

	if cessation != "uuuu" {
		log.Printf("ID %s already ceased, skipping", f.Id())
		return nil
	}

	json, err := sjson.Set(json, "properties.edtf:cessation", date.Format(edtfDateLayout))
	if err != nil {
		return err
	}

	json, err = sjson.Set(json, "properties.mz:is_current", 0)
	if err != nil {
		return err
	}

	return d.exportFeature(f.Id(), json)
}

func (d *WOFData) UpdateFeature(f geojson.Feature, pcData *onsdb.PostcodeData, pip *pipclient.PIPClient) error {
	bytes := f.Bytes()
	json := string(bytes)

	json, err := setDates(json, pcData)
	if err != nil {
		return err
	}

	// Don't set geometry for BT postcodes, because the licensing for these is more
	// restrictive. 🙄
	if !strings.HasPrefix(f.Name(), "BT") {
		json, err = setPointGeometry(json, pcData.Latitude, pcData.Longitude)
		if err != nil {
			return err
		}

		// If we've updated the geometry, set the source to OS
		json, err = sjson.Set(json, "properties.src:geom", "os")
		if err != nil {
			return err
		}

		json, err = setHierarchy(json, pip, pcData)
		if err != nil {
			return err
		}
	}

	json, err = setOSProperties(json, pcData)
	if err != nil {
		return err
	}

	return d.exportFeature(f.Id(), json)
}

func (d *WOFData) exportFeature(id string, json string) error {
	opts, err := options.NewDefaultOptions()
	if err != nil {
		return err
	}

	bytes := []byte(json)

	idint, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}

	path, err := uri.Id2AbsPath(d.dataPath, idint)
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

	return export.Export(bytes, opts, f)
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

func setPointGeometry(json string, latitude string, longitude string) (string, error) {
	json, err := sjson.SetRaw(json, "geometry.coordinates.0", longitude)
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
