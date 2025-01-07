package pipclient

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/whosonfirst/go-whosonfirst-spatial/database"
	"github.com/whosonfirst/go-whosonfirst-spatial/filter"

	reader "github.com/whosonfirst/go-reader"
	hierarchy "github.com/whosonfirst/go-whosonfirst-spatial/hierarchy"
	spr "github.com/whosonfirst/go-whosonfirst-spr/v2"
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

	db, err := database.NewRTreeSpatialDatabase(ctx, "rtree://")
	if err != nil {
		return nil, err
	}

	dir := os.DirFS(absPath)

	log.Print("Indexing PIP database")
	err = database.IndexDatabaseWithFS(ctx, db, dir)
	if err != nil {
		return nil, err
	}
	log.Print("Indexing PIP database complete")

	options := &hierarchy.PointInPolygonHierarchyResolverOptions{Database: db}

	resolver, err := hierarchy.NewPointInPolygonHierarchyResolver(ctx, options)
	if err != nil {
		return nil, err
	}

	readerUri := fmt.Sprintf("fs://%s", absPath)
	r, err := reader.NewReader(ctx, readerUri)
	if err != nil {
		return nil, err
	}

	resolver.SetReader(r)

	return &PIPClient{database: db, resolver: resolver}, nil
}

func (client *PIPClient) UpdateHierarchy(ctx context.Context, bytes []byte) ([]byte, error) {
	inputs := &filter.SPRInputs{IsCurrent: []int64{-1, 1}}

	// Only allow postalcode records in the admin hierarchy to be parented by the
	// following placetypes.
	resultsCallback := func(ctx context.Context, r reader.Reader, body []byte, possible []spr.StandardPlacesResult) (spr.StandardPlacesResult, error) {
		for _, item := range possible {
			switch item.Placetype() {
			case "locality", "localadmin", "county", "region":
				return item, nil
			}
		}

		return nil, nil
	}

	updateCallback := hierarchy.DefaultPointInPolygonHierarchyResolverUpdateCallback()

	_, newBytes, err := client.resolver.PointInPolygonAndUpdate(ctx, inputs, resultsCallback, updateCallback, bytes)

	return newBytes, err
}
