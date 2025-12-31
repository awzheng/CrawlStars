# CrawlStars

A concurrent web crawler and search engine built with Go and MongoDB Atlas. Crawls web pages, extracts content, and provides full-text search with relevance-based star ratings.

## What It Does

- Crawls web pages starting from a seed URL using concurrent workers
- Extracts page titles and the first 500 characters of body content
- Filters out navigation noise and short text fragments
- Stores data in MongoDB with unique URL constraint
- Provides a search API with fuzzy matching via MongoDB Atlas Search
- Rates search results on a 1-5 star scale based on absolute relevance scores
- Web interface with clean design and clickable result links

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

The crawler will:
- Start from the seed URL
- Use 10 concurrent workers to fetch and parse pages
- Extract links and continue crawling
- Stop automatically after 1000 pages (configurable in `internal/crawler/crawler.go`)
- Display progress with count, worker ID, URL, and page title
- Show final statistics including duration, success rate, and crawl speed

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

The interface features:
- Current configured seed URL display
- Search box with query input
- Results with clickable titles and URLs that open in new tabs
- Content snippets from crawled pages
- Integer star ratings (1-5) based on relevance

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

### Crawler Settings

Edit `internal/crawler/crawler.go` to change:
- `maxCrawls`: Maximum pages to crawl (default: 1000)
- Queue buffer size (default: 1000)
- Worker timeout (default: 5 seconds)

### Content Extraction

The crawler extracts:
- Page title from the `<title>` tag (or first 50 chars of content if no title)
- First 500 characters from the `<body>` element
- Filters out text fragments shorter than 10 characters to reduce navigation noise
- All `<a>` tag links for continued crawling

### Star Rating System

Search results are rated 1-5 stars based on MongoDB's absolute relevance score:

- 5 stars: Score >= 5.0 (excellent match - exact keyword matches)
- 4 stars: Score >= 4.0 (very good match - multiple keyword occurrences)
- 3 stars: Score >= 3.0 (good match - keyword present but not dominant)
- 2 stars: Score >= 2.0 (fair match - weak relevance)
- 1 star: Score < 2.0 (poor match - barely relevant)

You can adjust these thresholds in `internal/database/mongo.go` based on your dataset.

### Web Interface

The interface uses:
- Roboto font from Google Fonts
- Solarized Light color scheme (warm beige/cream background)
- Responsive design for desktop and mobile
- Clickable URLs that open in new tabs
- CSS variables at the top of `web/index.html` for easy customization

## Notes

- The database stores all pages from all crawl sessions
- Duplicate URLs are prevented by a unique index
- The crawler identifies itself with a User-Agent header to avoid being blocked
- Some sites may return 403 errors - this is normal behavior
- Search is case-insensitive with fuzzy matching for typos
- Environment variables are per-terminal session
- Connection includes a loading indicator showing elapsed time

## Troubleshooting

**Connection timeout**: The default timeout is 30 seconds. If you still have issues, check that your MongoDB URI is correct and your IP is whitelisted.

**403 errors during crawling**: Some sites block crawlers. Try different seed URLs or check if the site has a robots.txt policy.

**No search results**: Make sure the Atlas Search index is active and named "default". It can take 1-2 minutes to build after creation.

**IP blocked**: Add your current IP to MongoDB Atlas Network Access. Your IP may change if you're on a dynamic connection.

**Search shows old results**: The database persists across multiple crawl sessions. If you want to start fresh, drop the `webpages` collection in MongoDB Atlas.
