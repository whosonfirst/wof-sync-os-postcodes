package postalregionsdb

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/saracen/walker"
	"github.com/whosonfirst/go-whosonfirst-feature/properties"
)

type PostalRegion struct {
	Name      string
	WofID     int64
	Hierarchy []map[string]int64
}

type PostalRegionsDB struct {
	dataPath *string
	Regions  map[string]*PostalRegion
}

func NewPostalRegionsDB(dataPath string) *PostalRegionsDB {
	db := &PostalRegionsDB{dataPath: &dataPath, Regions: make(map[string]*PostalRegion)}

	return db
}

func (db *PostalRegionsDB) Build() error {
	var mutex = &sync.RWMutex{}

	walkFn := func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".geojson") {
			return nil
		}

		if strings.Contains(path, "alt") {
			return nil
		}

		f, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		placetype, err := properties.Placetype(f)
		if err != nil {
			return err
		}

		if placetype != "postalregion" {
			return nil
		}

		name, err := properties.Name(f)
		if err != nil {
			return err
		}

		id, err := properties.Id(f)
		if err != nil {
			return err
		}

		hierarchy := properties.Hierarchies(f)

		mutex.Lock()
		db.Regions[name] = &PostalRegion{
			Name:      name,
			WofID:     id,
			Hierarchy: hierarchy,
		}
		mutex.Unlock()

		return nil
	}

	errorFn := walker.WithErrorCallback(func(path string, err error) error {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	})

	return walker.Walk(*db.dataPath, walkFn, errorFn)
}
