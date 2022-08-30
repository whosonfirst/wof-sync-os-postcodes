package pipclient

import (
	"context"
	"os"
	"strings"

	"github.com/saracen/walker"
	hierarchy "github.com/whosonfirst/go-whosonfirst-spatial-hierarchy"
	_ "github.com/whosonfirst/go-whosonfirst-spatial-rtree"
	"github.com/whosonfirst/go-whosonfirst-spatial/database"
	"github.com/whosonfirst/go-whosonfirst-spatial/filter"
)

type PIPClient struct {
	database database.SpatialDatabase
	resolver *hierarchy.PointInPolygonHierarchyResolver
}

func NewPIPClient(ctx context.Context) (*PIPClient, error) {
	url := "rtree:///?strict=false"
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

func (client *PIPClient) BuildDatabase(ctx context.Context, path string) error {
	walkFn := func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".geojson") {
			return nil
		}

		f, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return client.database.IndexFeature(ctx, f)
	}

	errorFn := walker.WithErrorCallback(func(path string, err error) error {
		return err
	})

	return walker.Walk(path, walkFn, errorFn)

}

func (client *PIPClient) UpdateHierarchy(ctx context.Context, bytes []byte) ([]byte, error) {
	inputs := &filter.SPRInputs{}
	resultsCallback := hierarchy.FirstButForgivingSPRResultsFunc
	updateCallback := hierarchy.DefaultPointInPolygonHierarchyResolverUpdateCallback()

	_, newBytes, err := client.resolver.PointInPolygonAndUpdate(ctx, inputs, resultsCallback, updateCallback, bytes)

	return newBytes, err
}
