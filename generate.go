package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/cheggaaa/pb"
	"github.com/deckarep/golang-set"
)

func generateDatabase(databaseName string) {
	if _, err := os.Stat(databaseName + ".db"); err == nil {
		return
	}
	if _, err := os.Stat(databaseName + ".txt"); os.IsNotExist(err) {
		log.Fatal("In order to create " + databaseName + ".db you must have JSON file")
	}
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	var err error
	var db *bolt.DB
	db, err = bolt.Open(databaseName+".db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get the number of the lines for the progress bar
	var numberOfLines int
	numberOfLines, err = linesInFile(databaseName + ".txt")
	if err != nil {
		log.Fatal(err)
	}

	// Open the file for streaming JSON
	fmt.Println("Collecting all ingredients...")
	allIngredients := mapset.NewSet()
	file, err := os.Open(databaseName + ".txt")
	if err != nil {
		log.Fatal(err)
	}
	bar := pb.StartNew(numberOfLines)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dec := json.NewDecoder(strings.NewReader(scanner.Text()))
		bar.Increment()
		for {
			var m JSONLine
			if err = dec.Decode(&m); err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}
			for _, ingredient := range m.Ingredients {
				allIngredients.Add(ingredient)
			}
		}
	}
	file.Close()
	bar.FinishPrint("Finished loading.")
	if err = scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	fmt.Println("Creating buckets for each ingredient...")
	err = db.Update(func(tx *bolt.Tx) error {
		_, err1 := tx.CreateBucket([]byte("noingredients"))
		if err1 != nil {
			return fmt.Errorf("create bucket: %s", err1)
		}
		_, err1 = tx.CreateBucket([]byte("jsonlines"))
		if err1 != nil {
			return fmt.Errorf("create bucket: %s", err1)
		}
		for _, ingredient := range allIngredients.ToSlice() {
			_, err2 := tx.CreateBucket([]byte(ingredient.(string)))
			if err2 != nil {
				return fmt.Errorf("create bucket: %s", err2)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Inserting data into buckets...")
	file, err = os.Open(databaseName + ".txt")
	if err != nil {
		log.Fatal(err)
	}
	scanner = bufio.NewScanner(file)
	bar = pb.StartNew(numberOfLines)
	err = db.Update(func(tx *bolt.Tx) error {
		for scanner.Scan() {
			dec := json.NewDecoder(strings.NewReader(scanner.Text()))
			bar.Increment()
			for {
				var m JSONLine
				if err = dec.Decode(&m); err == io.EOF {
					break
				} else if err != nil {
					log.Fatal(err)
				}

				b := tx.Bucket([]byte("jsonlines"))
				if b == nil {
					return fmt.Errorf("doesn't exist")
				}
				id, _ := b.NextSequence()
				m.Text = strings.Split(strings.Split(strings.Split(m.Text, " - recipe -")[0], " | epicurious")[0], " - kraft recipes")[0]
				bJSON, errMarshal := json.Marshal(m)
				if errMarshal != nil {
					fmt.Println("error:", errMarshal)
				}
				b.Put(itob(id), bJSON)

				for _, ingredient := range m.Ingredients {
					b2 := tx.Bucket([]byte(ingredient))
					if b2 == nil {
						return fmt.Errorf("doesn't exist")
					}
					id2, _ := b2.NextSequence()
					b2.Put(itob(id2), itob(id))
				}

			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	file.Close()
	bar.FinishPrint("Finished loading.")
	if err = scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}

}
