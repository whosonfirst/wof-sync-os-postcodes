package hierarchy

import (
	"context"
	"fmt"
	"github.com/whosonfirst/go-reader"
	"github.com/whosonfirst/go-whosonfirst-spr/v2"
)

type FilterSPRResultsFunc func(context.Context, reader.Reader, []byte, []spr.StandardPlacesResult) (spr.StandardPlacesResult, error)

func FirstButForgivingSPRResultsFunc(ctx context.Context, r reader.Reader, body []byte, possible []spr.StandardPlacesResult) (spr.StandardPlacesResult, error) {

	if len(possible) == 0 {
		return nil, nil
	}

	parent_spr := possible[0]
	return parent_spr, nil
}

func FirstSPRResultsFunc(ctx context.Context, r reader.Reader, body []byte, possible []spr.StandardPlacesResult) (spr.StandardPlacesResult, error) {

	if len(possible) == 0 {
		return nil, fmt.Errorf("No results")
	}

	parent_spr := possible[0]
	return parent_spr, nil
}

func SingleSPRResultsFunc(ctx context.Context, r reader.Reader, body []byte, possible []spr.StandardPlacesResult) (spr.StandardPlacesResult, error) {

	if len(possible) != 1 {
		return nil, fmt.Errorf("Number of results != 1")
	}

	parent_spr := possible[0]
	return parent_spr, nil
}
