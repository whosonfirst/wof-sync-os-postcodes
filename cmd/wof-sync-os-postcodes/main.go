package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
	"github.com/whosonfirst/wof-sync-os-postcodes/onsdb"
	"github.com/whosonfirst/wof-sync-os-postcodes/pipclient"
	"github.com/whosonfirst/wof-sync-os-postcodes/postalregionsdb"
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
	var noCreate = flag.Bool("no-create", false, "Set to disable the creation of new any features")
	var noUpdate = flag.Bool("no-update", false, "Set to disable the updating of existing features")
	var wofAdminDataPath = flag.String("wof-admin-data-path", "", "The path to the GB admin data directory")
	var prefixFilter = flag.String("prefix-filter", "", "Just do work on the postcode starting with the string")
	var ignoreRestrictiveLicenceFlag = flag.Bool("ignore-restrictive-licence", false, "Ignore the restrictive license on the Northern Ireland postcodes")
	flag.Parse()

	dryRun := *dryRunFlag
	ignoreRestrictiveLicence := *ignoreRestrictiveLicenceFlag

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

	log.Print("Building postalregions database")
	regionDB := postalregionsdb.NewPostalRegionsDB(*wofAdminDataPath)
	err = regionDB.Build()
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Finished building postalregions database")

	pip, err := pipclient.NewPIPClient(ctx, *wofAdminDataPath)
	if err != nil {
		log.Fatal(err)
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

		// Track which postcodes we've seen, so we can make new ones later on
		seenPostcodesMutex.Lock()
		seenPostcodes[postcode] = true
		seenPostcodesMutex.Unlock()

		// We're doing updating existing postcodes in this pass, so skip the rest
		if *noUpdate {
			return nil
		}

		// Check whether the postcode match the prefix-filter flag, and skip if not
		if prefixFilter != nil && !strings.HasPrefix(postcode, *prefixFilter) {
			return nil
		}

		id := ""
		idResult := gjson.GetBytes(f, "id")
		if idResult.Exists() {
			id = idResult.String()
		}

		if id == "" {
			return fmt.Errorf("id not found on existing record with name %s", postcode)
		}

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

		changed, err := wof.UpdateFeature(ctx, f, postcodeData, regionDB, pip, dryRun, ignoreRestrictiveLicence)
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

	if *noCreate {
		log.Printf("no-create flag enabled, so skipping new postcodes")
	} else {
		log.Printf("Seen %d postcodes, now checking for new postcodes", len(seenPostcodes))

		onsCB := func(pc *onsdb.PostcodeData) error {
			// Skip if we've already seen this postcode
			if seenPostcodes[pc.Postcode] {
				return nil
			}

			if prefixFilter != nil && !strings.HasPrefix(pc.Postcode, *prefixFilter) {
				return nil
			}

			if !shouldCreateNewPostcode(pc) {
				log.Printf("Skipping new postcode we're not creating: %s", pc.Postcode)
				return nil
			}

			log.Printf("Creating new postcode: %s", pc.Postcode)
			atomic.AddUint64(&newCounter, 1)
			return wof.NewFeature(ctx, pc, regionDB, pip, dryRun)
		}

		err = db.Iterate(onsCB)
		if err != nil {
			log.Fatal(err)
		}
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
