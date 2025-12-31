package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/andrewzheng/CrawlStars/internal/database"
)

var db *database.DB

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Show what we're connecting to (helpful for debugging)
	displayURI := mongoURI
	if strings.Contains(mongoURI, "@") {
		// Redact password for security
		parts := strings.Split(mongoURI, "@")
		if len(parts) == 2 {
			credPart := strings.Split(parts[0], "://")
			if len(credPart) == 2 {
				displayURI = credPart[0] + "://***@" + parts[1]
			}
		}
	}
	log.Printf("🔌 Connecting to: %s", displayURI)

	var err error
	db, err = database.Connect(mongoURI)
	if err != nil {
		log.Fatalf("🔥 Failed to connect to DB: %v", err)
	}
	defer db.Disconnect()
	log.Println("✅ Database connected. Server listening on port " + port)

	// Serve the frontend HTML
	http.HandleFunc("/", homeHandler)

	// API endpoint for search
	http.HandleFunc("/search", corsMiddleware(searchHandler))

	// API endpoint to get current configuration
	http.HandleFunc("/info", corsMiddleware(infoHandler))

	log.Printf("🌐 Open your browser to: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// homeHandler serves the frontend HTML page
func homeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

// corsMiddleware adds CORS headers so the frontend can call the API
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Please provide a 'q' query parameter.", http.StatusBadRequest)
		return
	}

	log.Printf("🔎 Searching for: %s", query)

	results, err := db.SearchPages(query)
	if err != nil {
		log.Printf("💥 Search error: %v", err)
		http.Error(w, "Something exploded internally.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	seedURL := os.Getenv("SEED_URL")
	if seedURL == "" {
		seedURL = "Not configured"
	}

	mongoURI := os.Getenv("MONGO_URI")
	cluster := "localhost"
	if strings.Contains(mongoURI, "@") {
		parts := strings.Split(mongoURI, "@")
		if len(parts) == 2 {
			cluster = strings.Split(parts[1], "/")[0]
		}
	}

	info := map[string]string{
		"seedUrl": seedURL,
		"cluster": cluster,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
