package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Page holds the data for a crawled webpage.
type Page struct {
	URL     string  `bson:"url"`
	Title   string  `bson:"title"`
	Content string  `bson:"content"`
	Score   float64 `bson:"score,omitempty"` // MongoDB's search score (for internal use)
}

// SearchResult wraps the page with a star rating.
type SearchResult struct {
	Title   string `bson:"title" json:"title"`
	URL     string `bson:"url" json:"url"`
	Stars   int    `bson:"stars" json:"stars"` // Simple 1-5 integer rating
	Snippet string `bson:"snippet" json:"snippet"`
}

// DB holds the keys to the kingdom (MongoDB client).
type DB struct {
	client *mongo.Client
	coll   *mongo.Collection
}

// Connect connects to the database. Don't forget to unplug it later!
func Connect(uri string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Show a nice loading indicator with elapsed time
	done := make(chan bool)
	go func() {
		start := time.Now()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		fmt.Print("⏳ Connecting to database")
		for {
			select {
			case <-done:
				fmt.Print("\r\033[K") // Clear the line
				return
			case <-ticker.C:
				elapsed := time.Since(start).Seconds()
				// \r returns to start of line, \033[K clears to end of line
				fmt.Printf("\r⏳ Connecting to database... (%0.1fs elapsed)", elapsed)
			}
		}
	}()

	// Knock knock. Who's there? Mongo.
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		close(done)
		time.Sleep(50 * time.Millisecond) // Let the goroutine clean up
		return nil, fmt.Errorf("failed to knock on Mongo's door: %w", err)
	}

	// Ping it just to be sure it's awake.
	if err := client.Ping(ctx, nil); err != nil {
		close(done)
		time.Sleep(50 * time.Millisecond)
		return nil, fmt.Errorf("mongo is sleeping (ping failed): %w\n💡 Tip: Make sure MONGO_URI is set to your Atlas connection string!", err)
	}

	// Stop the loading indicator
	close(done)
	time.Sleep(50 * time.Millisecond) // Let the goroutine clean up

	// We're in! Prepare the collection.
	coll := client.Database("crawlstars").Collection("webpages")

	// Ensure the URL is unique. duplicated content is so last season.
	_, _ = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "url", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	return &DB{client: client, coll: coll}, nil
}

// Disconnect says goodbye to the database.
func (db *DB) Disconnect() error {
	return db.client.Disconnect(context.Background())
}

// InsertPage tucks a webpage into the database for safe keeping.
// If it already exists, we give it a makeover (update).
func (db *DB) InsertPage(page Page) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"url": page.URL}
	update := bson.M{"$set": page}

	_, err := db.coll.UpdateOne(context.Background(), filter, update, opts)
	return err
}

// SearchPages looks for pages matching the query using Atlas Search.
// It also awards Michelin stars based on relevance.
func (db *DB) SearchPages(query string) ([]SearchResult, error) {
	// The magic pipeline.
	// Requires an Atlas Search index named "default" on the 'webpages' collection.
	pipeline := mongo.Pipeline{
		{{Key: "$search", Value: bson.D{
			{Key: "index", Value: "default"},
			{Key: "text", Value: bson.D{
				{Key: "query", Value: query},
				{Key: "path", Value: bson.D{
					{Key: "wildcard", Value: "*"},
				}},
				{Key: "fuzzy", Value: bson.D{}}, // For when your spelling is... creative.
			}},
		}}},
		// Limit to top 10 candidates
		{{Key: "$limit", Value: 10}},
		// Project the score so we can judge them.
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "url", Value: 1},
			{Key: "title", Value: 1},
			{Key: "content", Value: 1},
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "searchScore"}}},
		}}},
	}

	cursor, err := db.coll.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, fmt.Errorf("search pipeline exploded: %w", err)
	}
	defer cursor.Close(context.Background())

	var results []Page
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return []SearchResult{}, nil
	}

	// Convert results to SearchResult with ABSOLUTE star ratings
	// Instead of normalizing to the top result, we rate each result independently
	// based on its raw MongoDB relevance score.
	//
	// STAR RATING SCALE (based on MongoDB Atlas Search scores):
	// ============================================================
	// 5 stars: Score >= 5  (Excellent match - exact keyword matches, boosted fields)
	// 4 stars: Score >= 4   (Very good match - multiple keyword occurrences)
	// 3 stars: Score >= 3   (Good match - keyword present but not dominant)
	// 2 stars: Score >= 2   (Fair match - weak relevance, partial matches)
	// 1 star:  Score < 2    (Poor match - barely relevant, edge case matches)
	//
	// Note: MongoDB Atlas Search scores are unbounded and can go higher than 10.
	// The scale focuses on practical ranges observed in typical searches.
	// You can adjust these thresholds based on your dataset and search behavior.

	var starred []SearchResult

	for _, p := range results {
		// Calculate stars based on absolute score thresholds
		// Simple linear scale: 5, 4, 3, 2, or 1 stars
		var stars int

		if p.Score >= 5.0 {
			stars = 5
		} else if p.Score >= 4.0 {
			stars = 4
		} else if p.Score >= 3.0 {
			stars = 3
		} else if p.Score >= 2.0 {
			stars = 2
		} else {
			stars = 1
		}

		// Create a snippet (first 100 chars of content)
		snippet := p.Content
		if len(snippet) > 100 {
			snippet = snippet[:100] + "..."
		}

		starred = append(starred, SearchResult{
			Title:   p.Title,
			URL:     p.URL,
			Stars:   stars,
			Snippet: snippet,
		})
	}

	return starred, nil
}
