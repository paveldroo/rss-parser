package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"io"
	"log"
	"net/http"
	"os"
)

type Entry struct {
	Link struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Thumbnail struct {
		URL string `xml:"url,attr"`
	} `xml:"thumbnail"`
	Title string `xml:"title"`
}

type Feed struct {
	Entries []Entry `xml:"entry"`
}

type Request struct {
	URL string `json:"url"`
}

var client *mongo.Client
var ctx context.Context

func init() {
	ctx = context.Background()
	client, _ = mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Connected to MongoDB")
}

func main() {
	router := gin.Default()
	router.POST("/parse", ParserHandler)
	log.Fatal(router.Run(":6000"))
}

func ParserHandler(c *gin.Context) {
	var request Request

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	entries, err := GetFeedEntries(request.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	fmt.Println(collection)
	for _, entry := range entries[2:] {
		res, err := collection.InsertOne(ctx, bson.M{
			"title":     entry.Title,
			"thumbnail": entry.Thumbnail.URL,
			"url":       entry.Link.Href,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		fmt.Println(res.InsertedID)
	}

	c.JSON(http.StatusOK, entries)
}

func GetFeedEntries(url string) ([]Entry, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36(KHTML, like Gecko) Chrome/70.0.3538.110Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	byteValue, _ := io.ReadAll(resp.Body)
	var feed Feed
	xml.Unmarshal(byteValue, &feed)
	return feed.Entries, nil
}
