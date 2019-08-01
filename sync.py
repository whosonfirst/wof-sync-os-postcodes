#!/usr/bin/env python
import csv
import glob
import logging
import os
import pprint
import re
import geojson
import json
from datetime import datetime, date

from ukpostcodeutils import validation

import shapely.geometry
from shapely.geometry import Point

import mapzen.whosonfirst.utils
import mapzen.whosonfirst.uri
import mapzen.whosonfirst.export


def main():
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument(
        "-s",
        "--source",
        dest="source",
        action="store",
        default=None,
        help="Path to the ONS CSV file",
    )
    parser.add_argument(
        "-d",
        "--dest",
        dest="dest",
        action="store",
        default=None,
        help="Directory of whosonfirst-data-postalcode-gb data to sync against",
    )
    parser.add_argument(
        "-v", "--verbose", dest="verbose", action="store_true")
    args = parser.parse_args()

    if args.verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)

    ons_path = os.path.abspath(args.source)
    wof_path = os.path.abspath(args.dest)

    # Build the big ol' dicts
    ons_postcode_data_lookup = build_ons_lookup(ons_path)
    (wof_postcode_id_lookup, invalid_wof_name_id_lookup) = build_wof_lookup(wof_path)

    # Start doing stuff to the WOF files
    deprecate_invalid_postcodes(invalid_wof_name_id_lookup, wof_path)
    cease_removed_postcodes(wof_postcode_id_lookup,
                            ons_postcode_data_lookup, wof_path)

    update_postcodes(wof_postcode_id_lookup,
                     ons_postcode_data_lookup, wof_path)


def deprecate_invalid_postcodes(invalid_wof_name_id_lookup, wof_path):
    invalid_postcode_ids = invalid_wof_name_id_lookup.values()
    logging.debug("Found %d invalid postcodes" % len(invalid_postcode_ids))

    for id in invalid_postcode_ids:
        deprecate_id(id, wof_path)


def cease_removed_postcodes(wof_postcode_id_lookup, ons_postcode_data_lookup, wof_path):
    ons_postcodes_set = set(ons_postcode_data_lookup.keys())
    wof_postcodes_set = set(wof_postcode_id_lookup.keys())

    removed_postcodes = wof_postcodes_set - ons_postcodes_set
    logging.debug("Found %d postcodes to remove" % len(removed_postcodes))

    for postcode in removed_postcodes:
        id = wof_postcode_id_lookup[postcode]
        cease_id(id, wof_path)


def update_postcodes(wof_postcode_id_lookup, ons_postcode_data_lookup, wof_path):
    ons_postcodes_set = set(ons_postcode_data_lookup.keys())
    wof_postcodes_set = set(wof_postcode_id_lookup.keys())

    updated_postcodes = wof_postcodes_set.intersection(ons_postcodes_set)

    for postcode in updated_postcodes:
        ons_data = ons_postcode_data_lookup[postcode]
        wof_id = wof_postcode_id_lookup[postcode]

        feature = load_feature(wof_id, wof_path)

        longitude = float(ons_data["longitude"])
        latitude = float(ons_data["latitude"])

        point = Point(longitude, latitude)
        feature["geometry"] = point

        props = feature["properties"]

        props["edtf:inception"] = convert_ons_date_to_edtf(
            ons_data["start"])
        props["edtf:cessation"] = convert_ons_date_to_edtf(
            ons_data["end"])

        if props["edtf:cessation"] == "uuuu":
            props["mz:is_current"] = 1
        else:
            props["mz:is_current"] = 0

        feature["properties"] = props

        # From: https://github.com/whosonfirst/whosonfirst-cookbook/blob/master/how_to/fixing_geometries.md
        feature = json.loads(geojson.dumps(feature))

        update_feature(feature, wof_path)


def build_ons_lookup(path):
    lookup = {}
    total = 0
    loaded = 0
    skipped = 0

    def log_progress():
        logging.debug(
            "Scanned %d ONS postcodes, loaded %d, skipped %d" % (
                total, loaded, skipped)
        )

    log_progress()

    with open(path, "r") as csvfile:
        reader = csv.DictReader(csvfile)

        for row in reader:
            total += 1
            postcode = row["pcds"]

            # Don't load BT postcodes (all of Northern Ireland), because the
            # licensing is restrictive.
            if postcode.startswith("BT"):
                skipped += 1
                continue

            data = {
                "latitude": row["lat"],
                "longitude": row["long"],
                "start": row["dointr"],
                "end": row["doterm"]
            }

            lookup[postcode] = data
            loaded += 1

            if total % 1000 == 0:
                log_progress()

    log_progress()
    return lookup


def build_wof_lookup(path):
    wof_lookup = {}
    invalid_wof_lookup = {}

    total = 0
    loaded = 0
    invalid = 0
    skipped = 0

    def log_progress():
        logging.debug(
            "Scanned %d WOF postcodes, loaded %d, skipped %d, found %d invalid" % (
                total, loaded, skipped, invalid)
        )

    log_progress()

    for feature in mapzen.whosonfirst.utils.crawl(path, inflate=True, ensure_placetype=["postalcode"]):
        total += 1

        properties = feature["properties"]

        # whosonfirst-data-postalcode-gb contains British dependencies, like
        # Bermuda, which have different postcode schemes. Ignore these for now.
        if properties["iso:country"] != "GB" or properties["wof:country"] != "GB":
            continue

        id = feature["id"]
        name = properties["wof:name"]
        postcode = normalise_postcode(name)

        if postcode is None:
            invalid += 1
            invalid_wof_lookup[name] = id
            continue

        # Ignore BT postcodes (all of Northern Ireland), because the licensing
        # is restrictive, and they don't feature in Code-Point Open.
        if postcode.startswith("BT"):
            skipped += 1
            continue

        wof_lookup[postcode] = id
        loaded += 1

        if total % 1000 == 0:
            log_progress()

    log_progress()

    return (wof_lookup, invalid_wof_lookup)


NON_ALPHA_RE = re.compile("[^A-Z0-9]+")
POSTCODE_RE = re.compile("^[A-Z]{1,2}[0-9]{1,2}[A-Z]? [0-9][A-Z]{2}$")


def normalise_postcode(postcode):
    """Return a normalised postcode if valid, or None if not."""

    postcode = NON_ALPHA_RE.sub("", postcode.upper())
    postcode = postcode[:-3] + " " + postcode[-3:]
    if POSTCODE_RE.match(postcode):
        return postcode
    return None


def cease_id(id, path):
    feature = load_feature(id, path)

    if feature["properties"].get("edtf:cessation", "uuuu") != "uuuu":
        logging.debug("ID %d already ceased", id)
        return

    # Mark as ceased when Code-Point Open was released
    feature["properties"]["edtf:cessation"] = "2019-05-uu"

    update_feature(feature, path)


def deprecate_id(id, path):
    feature = load_feature(id, path)

    if feature["properties"].get("edtf:deprecated", "uuuu") != "uuuu":
        logging.debug("ID %d already deprecated", id)
        return

    # Mark as deprecated today
    today = date.today()
    feature["properties"]["edtf:deprecated"] = today.isoformat()

    update_feature(feature, path)


def load_feature(id, path):
    feature_path = mapzen.whosonfirst.uri.id2abspath(path, id)
    feature = mapzen.whosonfirst.utils.load_file(feature_path)
    return feature


def update_feature(feature, path):
    exporter = mapzen.whosonfirst.export.flatfile(path)
    exporter.export_feature(feature)


def convert_ons_date_to_edtf(string):
    """Convert an ONS style month date (YYYYMM) to YYYY-MM-uu"""
    if string is None or string == "":
        return "uuuu"

    year = string[:4]
    month = string[4:]

    return "%s-%s-uu" % (year, month)


if __name__ == "__main__":
    main()
