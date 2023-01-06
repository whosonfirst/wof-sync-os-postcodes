package pipclient

import (
	"context"
	"fmt"
	"path/filepath"

	hierarchy "github.com/whosonfirst/go-whosonfirst-spatial-hierarchy"

	hierarchyFilter "github.com/whosonfirst/go-whosonfirst-spatial-hierarchy/filter"
	_ "github.com/aaronland/go-sqlite-mattn"
	_ "github.com/whosonfirst/go-whosonfirst-spatial-sqlite"
	"github.com/whosonfirst/go-whosonfirst-spatial/database"
	"github.com/whosonfirst/go-whosonfirst-spatial/filter"
)

type PIPClient struct {
	database database.SpatialDatabase
	resolver *hierarchy.PointInPolygonHierarchyResolver
}

func NewPIPClient(ctx context.Context, path string) (*PIPClient, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// URI scheme is still-unfortunately convoluted. It is the pairing
	// of the whosonfirst/go-whosonfirst-spatial/data scheme (sqlite://?dsn=)
	// and the aaronland/go-sqlite/v2 scheme (modernc://{STUFF} or mattn://{STUFF})
	// so we end up with sqlite://?dsn=modernc://{STUFF}. See also:
	// https://github.com/aaronland/go-sqlite/blob/main/database/dsn.go#L11
	// (20230106/thisisaaronland)
	
	// url := fmt.Sprintf("sqlite://?dsn=modernc://%s", absPath)

	url := fmt.Sprintf("sqlite://?dsn=mattn://%s", absPath)	
	db, err := database.NewSpatialDatabase(ctx, url)
	if err != nil {
		return nil, err
	}

	resolver, err := hierarchy.NewPointInPolygonHierarchyResolver(ctx, db, nil)
	if err != nil {
		return nil, err
	}

	return &PIPClient{database: db, resolver: resolver}, nil
}

func (client *PIPClient) UpdateHierarchy(ctx context.Context, bytes []byte) ([]byte, error) {
	inputs := &filter.SPRInputs{IsCurrent: []int64{-1, 1}}
	resultsCallback := hierarchyFilter.FirstButForgivingSPRResultsFunc
	updateCallback := hierarchy.DefaultPointInPolygonHierarchyResolverUpdateCallback()

	_, newBytes, err := client.resolver.PointInPolygonAndUpdate(ctx, inputs, resultsCallback, updateCallback, bytes)

	return newBytes, err
}
