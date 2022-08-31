package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
	"github.com/whosonfirst/wof-sync-os-postcodes/onsdb"
	"github.com/whosonfirst/wof-sync-os-postcodes/pipclient"
	"github.com/whosonfirst/wof-sync-os-postcodes/postcodevalidator"
	"github.com/whosonfirst/wof-sync-os-postcodes/wofdata"

	export "github.com/whosonfirst/go-whosonfirst-export/v2"

	_ "github.com/aaronland/go-uid-proxy"
	_ "github.com/aaronland/go-uid-whosonfirst"
	id "github.com/whosonfirst/go-whosonfirst-id"
)

func main() {
	var onsCSVPath = flag.String("ons-csv-path", "", "The path to the ONS postcodes CSV")
	var onsDate = flag.String("ons-date", "", "The date of the ONS postalcodes CSV")
	var wofPostalcodesPath = flag.String("wof-postalcodes-path", "", "The path to the WOF postalcodes data")
	var dryRunFlag = flag.Bool("dry-run", false, "Set to true to do nothing")
	var noUpdateHierarchy = flag.Bool("no-update-hierarchy", false, "Set true to disable updating hierarchy on existing features")
	var wofAdminSqlitePath = flag.String("wof-admin-sqlite-path", "", "The path to the GB admin SQLite database, used for PIPing the records")
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

	onsDBDate, err := time.Parse("2006-01-02", *onsDate)
	if err != nil {
		log.Fatalf("Missing or invalid -ons-date flag - make sure you explicitly set the date of the ONS database you're syncing against: %s", err)
	}

	log.Print("Building ONS database")
	db := onsdb.NewONSDB(*onsCSVPath)
	err = db.Build()
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Finished building ONS database")

	// Use separate pipclients for creating and updating features, so we can
	// enable/disable them independently.
	createPip, err := pipclient.NewPIPClient(ctx, *wofAdminSqlitePath)
	if err != nil {
		log.Fatalf("Failed to create PIP client: %s", err)
	}

	var updatePip *pipclient.PIPClient
	if *noUpdateHierarchy {
		log.Print("Updating hierarchy for existing features is disabled")
	} else {
		updatePip = createPip
	}

	seenPostcodes := make(map[string]bool)
	seenPostcodesMutex := sync.RWMutex{}

	var ceasedCounter uint64
	var deprecatedCounter uint64
	var updatedCounter uint64
	var newCounter uint64

	cb := func(f []byte) error {
		postcode := ""
		nameResult := gjson.GetBytes(f, "properties.wof:name")
		if nameResult.Exists() {
			postcode = nameResult.String()
		}

		if postcode == "" {
			return errors.New("name not found on existing record")
		}

		id := ""
		idResult := gjson.GetBytes(f, "id")
		if idResult.Exists() {
			id = idResult.String()
		}

		if id == "" {
			return fmt.Errorf("id not found on existing record with name %s", postcode)
		}

		// Track which postcodes we've seen, so we can make new ones later on
		seenPostcodesMutex.Lock()
		seenPostcodes[postcode] = true
		seenPostcodesMutex.Unlock()

		country := ""
		countryResult := gjson.GetBytes(f, "properties.wof:country")
		if countryResult.Exists() {
			country = countryResult.String()
		}

		if country != "GB" {
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
		log.Fatalf("Iteration failed: %s", err)
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
		return wof.NewFeature(pc, createPip, dryRun)
	}

	err = db.Iterate(onsCB)
	if err != nil {
		log.Fatal(err)
	}

	ceased := atomic.LoadUint64(&ceasedCounter)
	deprecated := atomic.LoadUint64(&deprecatedCounter)
	updated := atomic.LoadUint64(&updatedCounter)
	new := atomic.LoadUint64(&newCounter)

	log.Printf("Stats: %d not found and ceased, %d found invalid then deprecated, %d updated, %d new", ceased, deprecated, updated, new)
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

func createExportOptions(ctx context.Context) (*export.Options, error) {
	uri := "proxy:///?provider=whosonfirst://&minimum=100&pool=memory%3A%2F%2F"
	cl, _ := id.NewProviderWithURI(ctx, uri)

	opts, err := export.NewDefaultOptionsWithProvider(ctx, cl)
	return opts, err
}
