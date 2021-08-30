# wof-sync-os-postcodes

This utility syncs the [Who’s on First](https://www.whosonfirst.org) UK postcode data against the Office of National Statistics' (ONS) [Postcode Directory](https://geoportal.statistics.gov.uk/search?collection=Dataset&sort=-modified&tags=PRD_ONSPD). It deprecates invalid postcodes, ceases ones that no longer exist, updates ones that have moved, and creates new ones if necessary.

It should be rerun whenever there's a new release of the Postcode Directory, which is usually quarterly.

It needs a directory containing [`whosonfirst-data-postalcode-gb`](https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb), and the most recent ONS Postcode Directory as a single CSV file.

It also needs a running [PIP server](https://github.com/whosonfirst/go-whosonfirst-pip-v2) containing the UK admin data, for building the Who’s on First hierarchy from coordinates provided in the ONS data. It defaults to finding this at `http://localhost:8080`, but can be set with a flag.

It will ignore the coordinates from Northern Irish postcodes (starting with BT) as these are under more restrictive licensing conditions than the rest of the UK, so aren't suitable for inclusion in Who’s on First. It will use the inception/cessation data, however.

## Requirements

- Golang 1.17

## Example

Build the binary with a simple `make`. Then:

```shell
./bin/wof-sync-os-postcodes -wof-postalcodes-path ../whosonfirst-data-postalcode-gb/data -ons-csv-path ../ons/ONSPD_MAY_2019_UK/Data/ONSPD_MAY_2019_UK.csv -ons-date 2019-05-01
```

## Performing the sync

The `whosonfirst-data-postalcode-gb` repo has a large number of small files, and performing the actual sync and subsequent git operations against the repo is fairly painful.

I suggest using a 64GB machine with a ram disk. The ramdisk provides tolerable IO performance, and brings time to perform a fresh sync down to few hours. In my experience 32GB isn't enough and you will experience out-of-memory crashes, losing all your progress, as the RAM disk will not persist.

`setup.sh` contains a script which performs much of the set up for you. It expects to be run in an empty, ephemeral VM, so if you're running on a machine you care about, please read the script carefully before executing.

After executing, copy the ONS directory CSV into `/mnt/wof`. To bring up the PIP server, open tmux or your favourite screen-like app, and in one window execute:

```shell
cd /mnt/wof
./wof-pip-server whosonfirst-data-admin-gb/data/
```

And in another window perform the sync with something like:

```shell
/mnt/wof
./wof-sync-os-postcodes -wof-postalcodes-path whosonfirst-data-postalcode-gb/data/ -ons-csv-path ONSPD_AUG_2021_UK.csv -ons-date 2021-08-01
```

Now find something else to do for a few hours. 

Assuming you're on an ephemeral VM, you will need to set your Git name and email before you commit your changes:

```shell
git config --global user.name "Foo Bar"
git config --global user.email "foo@bar.com"
```

Some tips:

* Perform the `git push` over HTTPS, as SSH connections to Github seem to drop while the repo is being prepared for push
* Disable Git garbage collection on the repo as this will probably kick in at some point and you will scream (`setup.sh` does this for you)

## See also

- https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
- https://github.com/whosonfirst-data/whosonfirst-data-admin-gb
- https://github.com/whosonfirst/go-whosonfirst-pip-v2
- http://geoportal.statistics.gov.uk
