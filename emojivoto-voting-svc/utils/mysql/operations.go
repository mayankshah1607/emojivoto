package mysql

import (
	"database/sql"
	"fmt"
	"log"
)

var (
	//DB is the instance of the database that the application shall use
	DB *sql.DB

	dbName    = "emojivoto"
	tableName = "votes"
)

//InitDB is used to initialize the database with an emojivoto table
func InitDB(port, host, user, password string) error {
	var err error

	// open new connection
	DB, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port))
	if err != nil {
		return fmt.Errorf("Error establishing connection: %s", err)
	}

	// drop any existing emojivoto db
	_, err = DB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
	if err != nil {
		return fmt.Errorf("Error dropping database %s: %s", dbName, err)
	}

	// create new emojivoto db
	_, err = DB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName))
	if err != nil {
		return fmt.Errorf("Error creating database %s: %s", dbName, err)
	}

	//use emojivoto db
	DB.Close()
	DB, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbName))
	if err != nil {
		return fmt.Errorf("Error selecting database %s: %s", dbName, err)
	}

	return nil
}

// InitTables is used to initialize the `votes` table where all the vote counts will be stored
func InitTables() error {
	_, err := DB.Exec(fmt.Sprintf("CREATE TABLE %s(emoji varchar(60) UNIQUE, count int);", tableName))
	if err != nil {
		return err
	}

	log.Printf("Successfully created new table - %s\n", tableName)
	return nil
}

// FetchRecordForEmoji returns a single row for a given emoji
func FetchRecordForEmoji(emoji string) (Result, error) {
	var result Result

	results, err := DB.Query(fmt.Sprintf("SELECT * FROM %s WHERE emoji=\"%s\";", tableName, emoji))
	if err != nil {
		return result, err
	}

	for results.Next() {
		if err := results.Scan(&result.Shortcode, &result.NumVotes); err != nil {
			return result, err
		}
	}
	return result, nil

}

// UpdateVoteForEmoji is used to increate the vote count for a single emoji
func UpdateVoteForEmoji(emoji string) error {
	v, err := FetchRecordForEmoji(emoji)
	if err != nil {
		return err
	}

	if v.NumVotes == 0 { // No record exists for this emoji
		_, err := DB.Query(fmt.Sprintf("INSERT INTO %s VALUES('%s', 1)", tableName, emoji))

		if err != nil {
			return err
		}
		return nil
	}

	// Update existing record
	_, err = DB.Query(fmt.Sprintf("UPDATE %s SET count=count+1 WHERE emoji='%s'", tableName, emoji))

	if err != nil {
		return err
	}
	return nil
}

// GetAllVotes is used to fetch all the existing votes from the database
func GetAllVotes() ([]*Result, error) {
	results := make([]*Result, 0)

	r, err := DB.Query(fmt.Sprintf("SELECT * FROM %s", tableName))

	if err != nil {
		return results, err
	}

	for r.Next() {
		var result Result
		if err := r.Scan(&result.Shortcode, &result.NumVotes); err != nil {
			return results, err
		}
		results = append(results, &result)
	}
	return results, nil

}
