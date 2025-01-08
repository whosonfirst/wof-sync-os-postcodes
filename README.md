# wof-sync-os-postcodes

This utility syncs the [Who’s on First](https://www.whosonfirst.org) UK postcode data against the Office of National Statistics' (ONS) [Postcode Directory](https://geoportal.statistics.gov.uk/search?collection=Dataset&sort=Date%20Created%7Ccreated%7Cdesc&tags=PRD_ONSPD). It deprecates invalid postcodes, ceases ones that no longer exist, updates ones that have moved, and creates new ones if necessary.

It should be rerun whenever there's a new release of the Postcode Directory, which is usually quarterly.

It needs a directory containing [`whosonfirst-data-postalcode-gb`](https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb), and the most recent ONS Postcode Directory as a single CSV file.

It also needs a copy of the UK admin data, for building the Who’s on First hierarchy from coordinates provided in the ONS data.

By default it will ignore the coordinates from Northern Irish postcodes (starting with BT) as these are under more restrictive licensing conditions than the rest of the UK, so aren't suitable for inclusion in Who’s on First. It will use the inception/cessation data, however. You can override this with `-ignore-restrictive-licence` if you have a licence for internal business use, but don't merge these changes into mainline WOF as it's not a permitted licence.

## Requirements

- Golang 1.23

## Example usage

To perform a sync you'll need the following dependencies in place:

- [`whosonfirst-data-admin-gb`](https://github.com/whosonfirst-data/whosonfirst-data-admin-gb/)
- [`whosonfirst-data-postalcode-gb`](https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb/)
- A copy of the ONS postcode directory CSV file (the URL for this changes on every release, but you can find it from the [ONS Open Geography Portal](https://geoportal.statistics.gov.uk))

Either download this repo and build the binary with `make`, or download the CLI tool binary directory from releases.

Then you can run the following to perform the sync.

```shell
wof-sync-os-postcodes -wof-postalcodes-path whosonfirst-data-postalcode-gb/data -ons-csv-path ONSPD_MAY_2019_UK.csv -ons-date 2019-05-01 -wof-admin-data-path whosonfirst-data-admin-gb/data
```

## Performing the full sync

The `whosonfirst-data-postalcode-gb` repo has a large number of small files, and performing the actual sync and subsequent git operations against the repo is fairly painful.

I suggest using a 32GB machine with a local NVME SSD disk. The NVME SSD provides tolerable IO performance, and brings time to perform a fresh sync down to few hours.

`setup.sh` contains a script which performs much of the set up for you. It expects to be run in an empty, ephemeral VM on Google Cloud Compute, so if you're running on a machine you care about, please read the script carefully before executing.

```shell
curl https://raw.githubusercontent.com/whosonfirst/wof-sync-os-postcodes/master/setup.sh -o setup.sh
chmod +x setup.sh
./setup.sh
```

Perform the sync with something like:

```shell
./wof-sync-os-postcodes -wof-postalcodes-path /mnt/data/whosonfirst-data-postalcode-gb/data/ -ons-csv-path ONSPD_AUG_2021_UK.csv -ons-date 2021-08-01 -wof-admin-data-path /mnt/data/whosonfirst-data-admin-gb/data/
```

Now find something else to do for a few hours.

Assuming you're on an ephemeral VM, you will need to set your Git name and email before you commit your changes:

```shell
git config --global user.name "Foo Bar"
git config --global user.email "foo@bar.com"
```

Some tips:

- Perform the `git push` over HTTPS, as SSH connections to Github seem to drop while the repo is being prepared for push
- Disable Git garbage collection on the repo as this will probably kick in at some point and you will scream (`setup.sh` does this for you)

## See also

- https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
- https://github.com/whosonfirst-data/whosonfirst-data-admin-gb
- http://geoportal.statistics.gov.uk
