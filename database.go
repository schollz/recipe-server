package main

import (
	"fmt"
	"os"

	"github.com/boltdb/bolt"
)

// JSONLine is the data in each line of the file
type JSONLine struct {
	Text        string
	Ingredients []string
}

// Buckets prints a list of all buckets.
func getKeysFromDatabase(databaseName string) ([]string, error) {
	var allIngredients []string
	if _, err := os.Stat(databaseName + ".db"); os.IsNotExist(err) {
		fmt.Println(err)
		return allIngredients, err
	}

	db, err := bolt.Open(databaseName+".db", 0600, nil)
	if err != nil {
		fmt.Println(err)
		return allIngredients, err
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			allIngredients = append(allIngredients, string(name))
			return err
		})
	})
	if err != nil {
		fmt.Println(err)
	}
	return allIngredients, err
}
