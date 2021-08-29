# wof-sync-os-postcodes

This utility syncs the [Who’s on First](https://www.whosonfirst.org) UK postcode data against the Office of National Statistics' (ONS) [Postcode Directory](https://geoportal.statistics.gov.uk/search?collection=Dataset&sort=-modified&tags=PRD_ONSPD). It deprecates invalid postcodes, ceases ones that no longer exist, updates ones that have moved, and creates new ones if necessary.

It should be rerun whenever there's a new release of the Postcode Directory, which is usually quarterly.

It needs a directory containing [`whosonfirst-data-postalcode-gb`](https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb), and the most recent ONS Postcode Directory as a single CSV file.

It also needs a running [PIP server](https://github.com/whosonfirst/go-whosonfirst-pip-v2) containing the UK admin data, for building the Who’s on First hierarchy from coordinates provided in the ONS data. It defaults to finding this at `http://localhost:8080`, but can be set with a flag.

It will ignore the coordinates from Northern Irish postcodes (starting with BT) as these are under more restrictive licensing conditions than the rest of the UK, so aren't suitable for inclusion in Who’s on First. It will use the inception/cessation data, however.

## Requirements

- Golang 1.17
- 6GB RAM (including running the UK admin data in a PIP server)

## Example

Build the binary with a simple `make`. Then:

```shell
./bin/wof-sync-os-postcodes -wof-postalcodes-path ../whosonfirst-data-postalcode-gb/data -ons-csv-path ../ons/ONSPD_MAY_2019_UK/Data/ONSPD_MAY_2019_UK.csv -ons-date 2019-05-01
```

## See also

- https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
- https://github.com/whosonfirst-data/whosonfirst-data-admin-gb
- https://github.com/whosonfirst/go-whosonfirst-pip-v2
- http://geoportal.statistics.gov.uk
