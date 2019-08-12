package wofdata

import (
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/onsdb"

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

func (d *WOFData) UpdateFeature(f geojson.Feature, pcData *onsdb.PostcodeData) error {
	bytes := f.Bytes()
	json := string(bytes)

	inception := convertStringToEDTF(pcData.Inception)
	json, err := sjson.Set(json, "properties.edtf:inception", inception)
	if err != nil {
		return err
	}

	cessation := convertSQLStringtoEDTF(pcData.Cessation)
	json, err = sjson.Set(json, "properties.edtf:cessation", cessation)
	if err != nil {
		return err
	}

	isCurrent := 1
	if pcData.Cessation.Valid {
		isCurrent = 0
	}

	json, err = sjson.Set(json, "properties.mz:is_current", isCurrent)
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

func convertSQLStringtoEDTF(s sql.NullString) string {
	if !s.Valid {
		return "uuuu"
	}

	return convertStringToEDTF(s.String)
}

func convertStringToEDTF(s string) string {
	t, err := time.Parse("200601", s)
	if err != nil {
		log.Fatalf("Failed to parse inception/cessation date %s: %s", s, err)
	}

	return t.Format(edtfDateLayout)
}
