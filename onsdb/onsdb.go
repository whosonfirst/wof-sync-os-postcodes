package onsdb

import (
	"database/sql"
	"log"
	"os"

	"github.com/jmoiron/sqlx"

	// Driver for sqlx
	_ "github.com/mattn/go-sqlite3"
)

// PostcodeData represents an individual postcode with its associated data
type PostcodeData struct {
	Postcode  string         `db:"pcds"`
	Latitude  string         `db:"lat"`
	Longitude string         `db:"long"`
	Inception string         `db:"dointr"`
	Cessation sql.NullString `db:"doterm"`
}

// ONSDB is a wrapper around an SQLite database containing the ONS Poscode Directory
type ONSDB struct {
	conn *sqlx.DB
}

// NewONSDB creates a new ONSDB at the path specified
func NewONSDB(path string) *ONSDB {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalln("No database at path")
		return nil
	}

	conn, err := sqlx.Connect("sqlite3", path)

	if err != nil {
		log.Fatalln(err)
	}

	onsdb := &ONSDB{
		conn: conn,
	}

	return onsdb
}

// GetPostcodeData returns the PostcodeData for the postcode provided
func (db *ONSDB) GetPostcodeData(postcode string) (*PostcodeData, error) {
	data := []PostcodeData{}
	err := db.conn.Select(&data, "SELECT pcds, lat, long, dointr, doterm FROM postcodes WHERE pcds = $1 LIMIT 1", postcode)
	if err != nil {
		return nil, err
	}

	if len(data) > 0 {
		return &data[0], nil
	}

	return nil, nil
}
