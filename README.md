# wof-sync-os-postcodes

This utility syncs the Who’s on First UK postcode data against the Office of National Statistics' (ONS) Postcode Directory. It deprecates invalid postcodes, ceases ones that no longer exist, updates ones that have moved, and creates new ones if necessary.

It should be rerun whenever there's a new release of the Postcode Directory.

It needs a directory containing [`whosonfirst-data-postalcode-gb`](https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb), and the most recent [ONS Postcode Directory](http://geoportal.statistics.gov.uk) as a single CSV file.

It also needs a [PIP server](https://github.com/whosonfirst/go-whosonfirst-pip-v2) running containing the UK admin data, for building the Who’s on First hierarchy from coordinates provided in the ONS data. It defaults to finding this at `http://localhost:8080`, but can be set with a flag.

It will ignore the coordinates from Northern Irish postcodes (starting with BT) as these are under more restrictive licensing conditions than the rest of the UK, so aren't suitable for inclusion in Who’s on First. It will use the inception/cessation data, however, to know which ones are current.

# Requirements

- Golang 1.12 (might compile on earlier versions, but definitely needs modules support)

# Example

Build the binary with a simple `make`. Then:

```shell
./bin/wof-sync-os-postcodes -wof-postalcodes-path ../whosonfirst-data-postalcode-gb/data -ons-csv-path ../ons/ONSPD_MAY_2019_UK/Data/ONSPD_MAY_2019_UK.csv
```

# See also

- https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
- https://github.com/whosonfirst-data/whosonfirst-data-admin-gb
- https://github.com/whosonfirst/go-whosonfirst-pip-v2
- http://geoportal.statistics.gov.uk
