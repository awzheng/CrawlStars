package main

import (
	"log"
	"os"
	"strconv"

	"github.com/andrewzheng/CrawlStars/internal/crawler"
	"github.com/andrewzheng/CrawlStars/internal/database"
)

func main() {
	// Configuration (Environment Variables or Defaults)
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	seedURL := os.Getenv("SEED_URL")
	if seedURL == "" {
		seedURL = "https://en.wikipedia.org/wiki/Computer_science" // As requested!
	}

	workerCountStr := os.Getenv("WORKERS")
	workerCount, err := strconv.Atoi(workerCountStr)
	if err != nil || workerCount == 0 {
		workerCount = 10
	}

	log.Println("🌟 CrawlStars is initializing...")
	log.Printf("Target Database: %s", mongoURI)
	log.Printf("Seed URL: %s", seedURL)
	log.Printf("Worker Count: %d", workerCount)

	// 1. Connect to DB
	db, err := database.Connect(mongoURI)
	if err != nil {
		log.Fatalf("🔥 Failed to connect to DB: %v", err)
	}
	defer db.Disconnect()
	log.Println("✅ Database connected.")

	// 2. Initialize Crawler
	c := crawler.New(db)

	// 3. Launch!
	c.Start(seedURL, workerCount)
}
