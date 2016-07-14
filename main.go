package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
)

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

func main() {

	generateDatabase("titles")
	generateDatabase("instructions")
	generateDatabase("ingredients")
	// check("markov_title.0")
	// check("titles")
	// fmt.Println(getRandom("titles", "", true))
	// fmt.Println(getRandom("ingredients", "apples"))
	// fmt.Println(getRandom("instructions", "apples"))

	recipeSetup()
	fmt.Println(listOfIngredients[0:10])
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.GET("/", func(c *gin.Context) {
		title, _ := getRandom("titles", "", true, time.Now().UnixNano())
		c.Redirect(302, "/recipe/"+title.Text)
	})
	router.GET("/recipe/:title", func(c *gin.Context) {
		title := c.Param("title")
		recipe, _ := generateRecipe(title)
		c.HTML(http.StatusOK, "recipe.html", gin.H{
			"title":        recipe.Title,
			"ingredients":  recipe.Ingredients,
			"instructions": recipe.Instructions,
		})
	})
	router.Run(":8015")
}
