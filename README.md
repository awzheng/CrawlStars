# CrawlStars

A concurrent web crawler and search engine built with Go and MongoDB Atlas during my 1A/1B winter break. This project explores database fundamentals, concurrent programming patterns, and full-text search implementation.

## ‼️ Read the DevLog ‼️

I documented the entire development process, technical decisions, and reasoning behind every design choice in the devlog. It covers the Write Path (crawler architecture), Read Path (search system), and all the lessons learned along the way. Fully authentic, no AI slop. I promise.

Check it out here: [devlog.md](devlog.md)

## What It Does

CrawlStars implements a scalable web crawler with a producer-consumer architecture using goroutines and buffered channels. It extracts page content, stores it in MongoDB Atlas, and provides full-text search with fuzzy matching and absolute relevance scoring.

## Architecture

### Write Path (Crawler)
![CrawlStars Write Path](assets/CrawlStars-write-path.png)

**Crawler (Write Path):**
- Spawns configurable concurrent workers (default 10) to crawl pages in parallel
- Implements thread-safe deduplication using sync.Map and atomic operations
- Extracts page titles and first 500 characters of body content
- Filters text fragments shorter than 10 characters to reduce navigation noise
- Uses User-Agent headers and 15-second timeouts for robust HTTP requests
- Stops automatically after 1000 pages (configurable)

### Read Path (Search)
![CrawlStars Read Path](assets/CrawlStars-read-path.png)

**Search Engine (Read Path):**
- MongoDB Atlas Search with aggregation pipelines
- Fuzzy query matching for typo tolerance
- Absolute star rating system (1-5 stars based on relevance scores)
- RESTful API with JSON responses
- CORS middleware for cross-origin deployment flexibility

**Web Interface:**
- Clean search interface with Solarized Light theme
- Displays clickable URLs, content snippets, and star ratings
- XSS protection via HTML escaping
- Responsive design with staggered result animations

## Prerequisites

- Go 1.16 or higher
- MongoDB Atlas account (free tier works fine)
- Internet connection for crawling

## Setup

### 1. MongoDB Atlas Configuration

Create a free MongoDB Atlas cluster and set up the search index:

1. Go to https://cloud.mongodb.com and create a cluster
2. Add your IP address to the Network Access whitelist (or allow 0.0.0.0/0 for testing)
3. Create a database user with read/write permissions
4. Get your connection string (it will look like `mongodb+srv://username:password@cluster.mongodb.net/`)

### 2. Create Atlas Search Index

In MongoDB Atlas:
1. Navigate to your cluster and click the "Search" tab
2. Click "Create Search Index"
3. Choose "JSON Editor"
4. Set database to `crawlstars` and collection to `webpages`
5. Name the index `default`
6. Use this index definition:

```json
{
  "mappings": {
    "dynamic": false,
    "fields": {
      "title": {
        "type": "string",
        "analyzer": "lucene.standard"
      },
      "content": {
        "type": "string",
        "analyzer": "lucene.standard"
      }
    }
  }
}
```

7. Click "Create Search Index" and wait for it to become active

### 3. Install Dependencies

```bash
go mod download
```

## Usage

### Running the Crawler

Set your environment variables:

```bash
export MONGO_URI="mongodb+srv://username:password@cluster.mongodb.net/?appName=CrawlStars"
export SEED_URL="https://en.wikipedia.org/wiki/Computer_science"
export WORKERS=10
```

Start crawling:

```bash
go run cmd/crawler/main.go
```

The crawler will start from the seed URL and use 10 concurrent workers (goroutines) to fetch and parse pages. It extracts links from each page and adds them to a buffered queue (capacity 1000) for continued crawling. The crawler stops automatically after processing 1000 pages, which is configurable in `internal/crawler/crawler.go` by changing the `maxCrawls` value. During execution, it displays progress with count, worker ID, URL, and page title for each processed page, and shows final statistics including total duration, success rate, and pages per minute.

### Running the Search Server

In a new terminal, set the MongoDB URI:

```bash
export MONGO_URI="mongodb+srv://username:password@cluster.mongodb.net/?appName=CrawlStars"
```

Optionally set the seed URL (for display purposes only):

```bash
export SEED_URL="https://en.wikipedia.org/wiki/Computer_science"
```

Start the server:

```bash
go run cmd/server/main.go
```

The server will start on port 8080 by default. You can change this:

```bash
export PORT=3000
go run cmd/server/main.go
```

### Using the Web Interface

Open your browser to http://localhost:8080

The interface displays the currently configured seed URL and MongoDB cluster at the top (via the `/info` endpoint). Users can enter search queries in the search box, which sends requests to the `/search` endpoint. Results are displayed with clickable titles and URLs (opening in new tabs), content snippets from crawled pages, and star ratings (1 to 5 stars) based on absolute relevance scores from MongoDB Atlas Search.

### Using the API Directly

Search for pages:

```bash
curl "http://localhost:8080/search?q=computer"
```

Get current configuration:

```bash
curl "http://localhost:8080/info"
```

## Project Structure

```
CrawlStars/
├── cmd/
│   ├── crawler/main.go    # Crawler entry point
│   └── server/main.go     # Search API server
├── internal/
│   ├── crawler/
│   │   └── crawler.go     # Concurrent crawler logic
│   └── database/
│       └── mongo.go       # MongoDB operations and search
├── web/
│   └── index.html         # Search interface
└── go.mod
```

## Configuration

## Configuration

### Crawler Settings

Edit `internal/crawler/crawler.go` to change the maximum number of pages to crawl (default is 1000 via `maxCrawls`), the Work Queue buffer size (default 1000), the Results Channel buffer size (default 100), or the HTTP client timeout (default 15 seconds in the `process()` function).

### Content Extraction

The crawler's `process()` function uses Go's HTML tokenizer to extract page content. It captures the page title from the `<title>` tag, the first 500 characters from the `<body>` element (filtering out text fragments shorter than 10 characters to reduce navigation noise), and all `<a>` tag links for continued crawling. If no title tag exists, it uses the first 50 characters of content as the title.

### Star Rating System

Search results receive 1 to 5 star ratings based on MongoDB Atlas Search absolute relevance scores, calculated in the `SearchPages()` function in `internal/database/mongo.go`:

**Absolute Scoring Scale:**
- 5 stars: Score >= 5.0 (excellent match with exact keyword matches)
- 4 stars: Score >= 4.0 (very good match with multiple keyword occurrences)
- 3 stars: Score >= 3.0 (good match where keyword is present but not dominant)
- 2 stars: Score >= 2.0 (fair match with weak relevance)
- 1 star: Score < 2.0 (poor match, barely relevant)

This absolute system was chosen over normalized scoring because it preserves comparability across different queries and accurately represents actual relevance rather than relative ranking.

### Web Interface Design

The frontend uses the Roboto font from Google Fonts and follows the Solarized Light color scheme with a warm beige/cream background. The design is responsive for both desktop and mobile viewing. All URLs are clickable and open in new tabs for user convenience. Colors and styling are defined using CSS variables at the top of `web/index.html` for easy customization. User input is sanitized with the `escapeHtml()` function to prevent XSS attacks. Results appear with a staggered animation delay for a polished loading effect.

## Technical Notes

The MongoDB database persists all pages across multiple crawl sessions. Duplicate URLs are prevented by a unique index on the `url` field, created during the `Connect()` function in `database/mongo.go`. The crawler identifies itself with a User-Agent header to avoid being blocked by websites. Some sites may still return 403 errors, which is normal behavior for crawler protection. MongoDB Atlas Search provides case-insensitive searching with built-in fuzzy matching for typo tolerance. Environment variables are per-terminal session and must be set in each terminal where you run commands. The database connection includes a loading indicator goroutine that displays elapsed time during connection setup.

## Troubleshooting

**Connection timeout**: The default timeout is 30 seconds. If you still have issues, check that your MongoDB URI is correct and your IP is whitelisted.

**403 errors during crawling**: Some sites block crawlers. Try different seed URLs or check if the site has a robots.txt policy.

**No search results**: Make sure the Atlas Search index is active and named "default". It can take 1-2 minutes to build after creation.

**IP blocked**: Add your current IP to MongoDB Atlas Network Access. Your IP may change if you're on a dynamic connection.

**Search shows old results**: The database persists across multiple crawl sessions. If you want to start fresh, drop the `webpages` collection in MongoDB Atlas.
