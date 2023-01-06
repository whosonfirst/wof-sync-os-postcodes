package pipclient

import (
	"context"
	"fmt"
	"path/filepath"

	hierarchy "github.com/whosonfirst/go-whosonfirst-spatial-hierarchy"

	hierarchyFilter "github.com/whosonfirst/go-whosonfirst-spatial-hierarchy/filter"

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

	url := fmt.Sprintf("sqlite://?dsn=%s", absPath)
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
