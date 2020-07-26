package main

import (
	"context"
	"flag"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tomtaylor/wof-sync-os-postcodes/onsdb"
	"github.com/tomtaylor/wof-sync-os-postcodes/pipclient"
	"github.com/tomtaylor/wof-sync-os-postcodes/postcodevalidator"
	"github.com/tomtaylor/wof-sync-os-postcodes/wofdata"

	exportOptions "github.com/whosonfirst/go-whosonfirst-export/options"
	geojson "github.com/whosonfirst/go-whosonfirst-geojson-v2"
	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"

	proxy "github.com/aaronland/go-artisanal-integers-proxy"
	pool "github.com/aaronland/go-pool"
	"github.com/whosonfirst/go-whosonfirst-id-proxy/provider"
)

func main() {
	var onsCSVPath = flag.String("ons-csv-path", "", "The path to the ONS postcodes CSV")
	var wofPostalcodesPath = flag.String("wof-postalcodes-path", "", "The path to the WOF postalcodes data")
	var pipHost = flag.String("pip-host", "http://localhost:8080/", "The host of the PIP server")
	var dryRunFlag = flag.Bool("dry-run", false, "Set to true to do nothing")
	var noUpdateHierarchy = flag.Bool("no-update-hierarchy", false, "Set true to disable updating hierarchy on existing features")
	flag.Parse()

	dryRun := *dryRunFlag

	if dryRun {
		log.Print("Performing dry run")
	}

	ctx := context.Background()
	opts, err := createExportOptions(ctx)
	if err != nil {
		log.Fatal(err)
	}

	wof := wofdata.NewWOFData(*wofPostalcodesPath, opts)

	log.Print("Building ONS database")

	onsDBDate := time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC)
	db := onsdb.NewONSDB(*onsCSVPath)
	err = db.Build()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Finished building ONS database")

	// Use separate pipclients for creating and updating features, so we can
	// enable/disable them independently.
	createPip := pipclient.NewPIPClient(*pipHost)
	var updatePip *pipclient.PIPClient
	if *noUpdateHierarchy {
		log.Print("Updating hierarchy for existing features is disabled")
	} else {
		updatePip = pipclient.NewPIPClient(*pipHost)
	}

	seenPostcodes := make(map[string]bool)
	seenPostcodesMutex := sync.RWMutex{}

	var ceasedCounter uint64
	var deprecatedCounter uint64
	var updatedCounter uint64
	var newCounter uint64

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

		postcodeData, err := db.GetPostcodeData(postcode)
		if err != nil {
			return err
		}

		if postcodeData == nil {
			// If we can't find the postcode in the database but it's valid, then cease it
			if postcodevalidator.Validate(postcode) {
				changed, err := wof.CeaseFeature(f, onsDBDate, dryRun)
				if changed {
					log.Printf("Ceased postcode not in ONS DB: %s (ID %s)", postcode, id)
					atomic.AddUint64(&ceasedCounter, 1)
				}

				if err != nil {
					return err
				}

				return nil
			}

			// If it's not valid, then deprecate it, as it probably should never have existed
			changed, err := wof.DeprecateFeature(f, dryRun)
			if changed {
				log.Printf("Deprecated invalid postcode: %s (ID %s)", postcode, id)
				atomic.AddUint64(&deprecatedCounter, 1)
			}

			if err != nil {
				return err
			}

			return nil
		}

		changed, err := wof.UpdateFeature(f, postcodeData, updatePip, dryRun)
		if changed {
			log.Printf("Updated postcode: %s (ID %s)", postcode, id)
			atomic.AddUint64(&updatedCounter, 1)
		}

		if err != nil {
			return err
		}

		return nil
	}

	log.Print("Walking over WOF postcodes")

	err = wof.Iterate(cb)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Seen %d postcodes, now checking for new postcodes", len(seenPostcodes))

	onsCB := func(pc *onsdb.PostcodeData) error {
		// Skip if we've already seen this postcode
		if seenPostcodes[pc.Postcode] {
			return nil
		}

		if !shouldCreateNewPostcode(pc) {
			log.Printf("Skipping new postcode we're not creating: %s", pc.Postcode)
			return nil
		}

		log.Printf("Creating new postcode: %s", pc.Postcode)
		atomic.AddUint64(&newCounter, 1)
		if dryRun {
			return nil
		}
		return wof.NewFeature(pc, createPip)
	}

	err = db.Iterate(onsCB)
	if err != nil {
		log.Fatal(err)
	}

	ceased := atomic.LoadUint64(&ceasedCounter)
	deprecated := atomic.LoadUint64(&deprecatedCounter)
	updated := atomic.LoadUint64(&updatedCounter)
	new := atomic.LoadUint64(&newCounter)

	log.Printf("Stats: %d ceased, %d deprecated, %d updated, %d new", ceased, deprecated, updated, new)
}

func shouldCreateNewPostcode(pc *onsdb.PostcodeData) bool {
	// Channel Islands
	if pc.CountryCode == "L93000001" {
		return false
	}

	// Isle of Man
	if pc.CountryCode == "M83000003" {
		return false
	}

	return true
}

func createExportOptions(ctx context.Context) (exportOptions.Options, error) {
	pl, err := pool.NewPool(ctx, "memory://")
	if err != nil {
		return nil, err
	}

	svcArgs := proxy.ProxyServiceArgs{
		BrooklynIntegers: true,
		MinCount:         100,
	}

	svc, err := proxy.NewProxyServiceWithPool(pl, svcArgs)
	if err != nil {
		return nil, err
	}

	pr, err := provider.NewProxyServiceProvider(svc)
	if err != nil {
		return nil, err
	}

	opts, err := exportOptions.NewDefaultOptionsWithProvider(pr)
	return opts, err
}
