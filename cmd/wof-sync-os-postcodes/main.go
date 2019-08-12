package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/onsdb"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/postcodevalidator"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/wofdata"
	geojson "github.com/whosonfirst/go-whosonfirst-geojson-v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"
)

func main() {

	var onsDBPath = flag.String("ons-db-path", "", "The path to the ONS postcodes sqlite database")
	var wofDataPath = flag.String("wof-data-path", "", "The path to the ONS postcodes sqlite database")
	flag.Parse()

	onsDBDate := time.Date(2019, time.May, 1, 0, 0, 0, 0, time.UTC)
	db := onsdb.NewONSDB(*onsDBPath)

	seenPostcodes := make(map[string]bool)
	seenPostcodesMutex := sync.RWMutex{}

	wof := wofdata.NewWOFData(*wofDataPath)

	cb := func(f geojson.Feature) error {
		postcode := f.Name()
		id := f.Id()

		// Track which postcodes we've seen, so we can make new ones later on
		seenPostcodesMutex.Lock()
		seenPostcodes[postcode] = true
		seenPostcodesMutex.Unlock()

		wofFeature, ok := f.(*feature.WOFFeature)
		if !ok {
			log.Printf("Skipping non-WOF feature: %s (ID %s)", postcode, id)
			return nil
		}

		spr, err := wofFeature.SPR()
		if err != nil {
			return err
		}

		if spr.Country() != "GB" {
			log.Printf("Skipping non-GB postcode: %s (ID %s)", postcode, id)
			return nil
		}

		if !postcodevalidator.Validate(postcode) {
			log.Printf("Deprecating invalid postcode: %s (ID %s)", postcode, id)
			return wof.DeprecateFeature(f)
		}

		postcodeData, err := db.GetPostcodeData(postcode)
		if err != nil {
			return err
		}

		if postcodeData == nil {
			log.Printf("Ceasing postcode not in ONS DB: %s (ID %s)", postcode, id)
			return wof.CeaseFeature(f, onsDBDate)
		}

		return wof.UpdateFeature(f, postcodeData)
	}

	err := wof.Iterate(cb)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Seen %d postcodes", len(seenPostcodes))
}
