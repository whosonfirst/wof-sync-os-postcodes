package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/onsdb"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/pipclient"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/postcodevalidator"
	"github.com/tomtaylor/whosonfirst-postalcode-gb-os-sync/wofdata"

	geojson "github.com/whosonfirst/go-whosonfirst-geojson-v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"
)

func main() {
	var onsCSVPath = flag.String("ons-csv-path", "", "The path to the ONS postcodes CSV")
	var wofPostalcodesPath = flag.String("wof-postalcodes-path", "", "The path to the WOF postalcodes data")
	var pipHost = flag.String("pip-host", "http://localhost:8080/", "The host of the PIP server")
	flag.Parse()

	log.Print("Building ONS database")

	onsDBDate := time.Date(2019, time.May, 1, 0, 0, 0, 0, time.UTC)
	db := onsdb.NewONSDB(*onsCSVPath)
	err := db.Build()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Finished building ONS database")

	pip := pipclient.NewPIPClient(*pipHost)

	seenPostcodes := make(map[string]bool)
	seenPostcodesMutex := sync.RWMutex{}

	wof := wofdata.NewWOFData(*wofPostalcodesPath)

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

		return wof.UpdateFeature(f, postcodeData, pip)
	}

	err = wof.Iterate(cb)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Seen %d postcodes", len(seenPostcodes))
}
