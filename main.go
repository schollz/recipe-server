package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/deckarep/golang-set"
	"gopkg.in/cheggaaa/pb.v1"
)

// JSONLine is the data in each line of the file
type JSONLine struct {
	Text        string
	Ingredients []string
}

// Recipe is the recipe JSON
type Recipe struct {
	Title        string
	Ingredients  []string
	Instructions []string
}

var listOfIngredients []string

func init() {
	listOfIngredients = getAllIngredients()
}

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func linesInFile(fileName string) (int, error) {
	// Count the number of lines in file
	file, err := os.Open(fileName)
	if err != nil {
		return -1, err
	}
	lines, _ := lineCounter(file)
	file.Close()
	return lines, nil
}

func generateDatabase(databaseName string) {

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

func check(databaseName string) {
	var err error
	var db *bolt.DB
	db, err = bolt.Open(databaseName+".db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("apples"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			b2 := tx.Bucket([]byte("jsonlines"))
			var m JSONLine
			err = json.Unmarshal(b2.Get(v), &m)
			if err != nil {
				return err
			}
			fmt.Printf("key=%v, value=%v, found=%v\n", k, v, m)
			fmt.Println(hasIngredients(m.Text))
		}
		return nil
	})
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

func getAllIngredients() []string {
	var ingredientsSorted []string
	allIngredients := mapset.NewSet()
	ingredients, _ := getKeysFromDatabase("titles")
	for _, ingredient := range ingredients {
		allIngredients.Add(ingredient)
	}
	ingredients, _ = getKeysFromDatabase("ingredients")
	for _, ingredient := range ingredients {
		allIngredients.Add(ingredient)
	}
	ingredients, _ = getKeysFromDatabase("instructions")
	for _, ingredient := range ingredients {
		allIngredients.Add(ingredient)
	}
	m := make(map[string]int)
	for _, ingredient := range allIngredients.ToSlice() {
		m[ingredient.(string)] = len(ingredient.(string))
	}

	n := map[int][]string{}
	var a []int
	for k, v := range m {
		n[v] = append(n[v], k)
	}
	for k := range n {
		a = append(a, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(a)))
	for _, k := range a {
		for _, s := range n[k] {
			ingredientsSorted = append(ingredientsSorted, s)
		}
	}
	return ingredientsSorted
}

func getRandom(databaseName string, ingredient string, mustHaveIngredients bool) (JSONLine, error) {
	var m JSONLine
	var err error
	var lastKey []byte
	var db *bolt.DB
	s1 := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s1)
	numberOfTries := 0

	db, err = bolt.Open(databaseName+".db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if ingredient == "" {
		ingredient = "jsonlines"
	}
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ingredient))
		if b == nil {
			return fmt.Errorf("No such bucket")
		}
		c := b.Cursor()
		lastKey, _ = c.Last()
		return nil
	})
	if err != nil {
		return m, fmt.Errorf("No data")
	}
	numberThings := binary.BigEndian.Uint64(lastKey)

	for numberOfTries < 30 {
		chosenNumber := uint64(0)
		if numberThings > 1 {
			chosenNumber = uint64(r.Intn(int(numberThings - 1)))
		}
		err = db.View(func(tx *bolt.Tx) error {
			if ingredient != "jsonlines" {
				b0 := tx.Bucket([]byte(ingredient))
				chosenID := b0.Get(itob(chosenNumber))
				b := tx.Bucket([]byte("jsonlines"))
				err = json.Unmarshal(b.Get(chosenID), &m)
			} else {
				b := tx.Bucket([]byte("jsonlines"))
				err = json.Unmarshal(b.Get(itob(chosenNumber)), &m)
			}
			if err != nil {
				return err
			}
			return nil
		})
		if m.Text == "" && len(m.Ingredients) == 0 {
			err = fmt.Errorf("No data")
			numberOfTries++
		} else if len(m.Ingredients) == 0 && mustHaveIngredients {
			err = fmt.Errorf("Doesn't have ingredients")
			numberOfTries++
		} else {
			numberOfTries += 1000
		}
	}

	return m, err
}

func generateRecipe() (Recipe, error) {
	var recipe Recipe
	allIngredients := mapset.NewSet()
	title, err := getRandom("titles", "", true)
	recipe.Title = title.Text
	for _, ingredient := range title.Ingredients {
		allIngredients.Add(ingredient)
	}

	for _, ingredient := range allIngredients.ToSlice() {
		instruction, err2 := getRandom("instructions", ingredient.(string), true)
		if err2 == nil {
			recipe.Instructions = append(recipe.Instructions, instruction.Text)
		}
		for _, insIngredient := range instruction.Ingredients {
			allIngredients.Add(insIngredient)
		}
	}

	for i := 0; i < 2; i++ {
		instruction, err2 := getRandom("instructions", "", false)
		if err2 == nil {
			recipe.Instructions = append(recipe.Instructions, instruction.Text)
		}
		for _, insIngredient := range instruction.Ingredients {
			allIngredients.Add(insIngredient)
		}
	}

	for _, ingredient := range allIngredients.ToSlice() {
		ingredient, err2 := getRandom("ingredients", ingredient.(string), true)
		if err2 == nil {
			recipe.Ingredients = append(recipe.Ingredients, ingredient.Text)
		}
	}

	// fmt.Println("\n\nTitle: " + strings.Title(recipe.Title))
	// fmt.Println("\nIngredients:\n")
	// for _, ingredient := range recipe.Ingredients {
	// 	fmt.Println("- " + ingredient)
	// }
	// fmt.Println("\nInstructions:\n")
	// for num, instruction := range recipe.Instructions {
	// 	fmt.Printf("%d. %s\n", num+1, instruction)
	// }
	return recipe, err
}

const delim = "?!.;,*"

func isDelim(c string) bool {
	if strings.Contains(delim, c) {
		return true
	}
	return false
}

func cleanString(input string) string {

	size := len(input)
	temp := ""
	var prevChar string

	for i := 0; i < size; i++ {
		//fmt.Println(input[i])
		str := string(input[i]) // convert to string for easier operation
		if (str == " " && prevChar != " ") || !isDelim(str) {
			temp += str
			prevChar = str
		} else if prevChar != " " && isDelim(str) {
			temp += " "
		}
	}
	return temp
}

func hasIngredients(text string) []string {
	var ingredients []string
	text = " " + cleanString(strings.ToLower(text)) + " "
	for _, ingredient := range listOfIngredients {
		if ingredient == "recipe" || ingredient == "jsonlines" {
			continue
		}
		if strings.Contains(text, " "+ingredient+" ") {
			ingredients = append(ingredients, ingredient)
			text = strings.Replace(text, " "+ingredient+" ", " ", -1)
		}
	}
	return ingredients
}

func main() {

	//generateDatabase("titles")
	//generateDatabase("instructions")
	//generateDatabase("ingredients")
	// check("markov_title.0")
	check("titles")
	// fmt.Println(getRandom("titles", "", true))
	// fmt.Println(getRandom("ingredients", "apples"))
	// fmt.Println(getRandom("instructions", "apples"))

	fmt.Println(listOfIngredients[0:10])
	// router := gin.Default()
	// router.LoadHTMLGlob("templates/*")
	// router.GET("/", func(c *gin.Context) {
	// 	recipe, _ := generateRecipe()
	// 	c.HTML(http.StatusOK, "recipe.html", gin.H{
	// 		"title":        recipe.Title,
	// 		"ingredients":  recipe.Ingredients,
	// 		"instructions": recipe.Instructions,
	// 	})
	// })
	// router.Run(":8015")
}
