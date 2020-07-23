module github.com/tomtaylor/wof-sync-os-postcodes

go 1.12

require (
	github.com/aaronland/go-artisanal-integers-proxy v0.2.4
	github.com/aaronland/go-pool v0.0.0-20191128211702-88306299c758
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/smartystreets/scanners v1.0.1
	github.com/tidwall/gjson v1.6.0
	github.com/tidwall/sjson v1.0.4
	github.com/whosonfirst/go-whosonfirst-export v0.3.1
	github.com/whosonfirst/go-whosonfirst-geojson-v2 v0.12.3
	github.com/whosonfirst/go-whosonfirst-id-proxy v0.0.1
	github.com/whosonfirst/go-whosonfirst-index v0.3.1
	github.com/whosonfirst/go-whosonfirst-uri v0.2.0
	github.com/whosonfirst/warning v0.1.1 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
)

replace github.com/tidwall/gjson => github.com/tidwall/gjson v1.3.5
