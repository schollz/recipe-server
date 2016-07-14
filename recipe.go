package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/deckarep/golang-set"
)

// Recipe is the recipe JSON
type Recipe struct {
	Title        string
	Ingredients  []string
	Instructions []string
}

var listOfIngredients []string

func recipeSetup() {
	listOfIngredients = getAllIngredients()
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
