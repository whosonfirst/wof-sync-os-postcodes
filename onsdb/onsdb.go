package onsdb

import (
	"log"
	"os"

	"github.com/smartystreets/scanners/csv"
)

// PostcodeData represents an individual postcode with its associated data
type PostcodeData struct {
	Postcode          string `csv:"pcds"`
	Latitude          string `csv:"lat"`
	Longitude         string `csv:"long"`
	Inception         string `csv:"dointr"`
	Cessation         string `csv:"doterm"`
	CountryCode       string `csv:"ctry"`
	RegionCode        string `csv:"rgn"`
	CountyCode        string `csv:"oscty"`
	DistrictCode      string `csv:"oslaua"`
	PositionalQuality string `csv:"osgrdind"`
}

// ONSDB is a wrapper around an SQLite database containing the ONS Poscode Directory
type ONSDB struct {
	data map[string]*PostcodeData
	path string
}

// NewONSDB creates a new ONSDB at the path specified
func NewONSDB(path string) *ONSDB {
	data := make(map[string]*PostcodeData)
	return &ONSDB{path: path, data: data}
}

func (db *ONSDB) Build() error {
	f, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner, err := csv.NewStructScanner(f)
	if err != nil {
		log.Panic(err)
	}

	for scanner.Scan() {
		var pcData PostcodeData
		if err := scanner.Populate(&pcData); err != nil {
			return err
		}

		db.data[pcData.Postcode] = &pcData
	}

	return scanner.Error()
}

// GetPostcodeData returns the PostcodeData for the postcode provided
func (db *ONSDB) GetPostcodeData(postcode string) (*PostcodeData, error) {
	pcData := db.data[postcode]
	return pcData, nil
}

func (db *ONSDB) Iterate(cb func(*PostcodeData) error) error {
	for _, pc := range db.data {
		err := cb(pc)
		if err != nil {
			return err
		}
	}

	return nil
}
