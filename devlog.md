# CrawlStars DevLog

> Never have I ever... felt so passionate before about a database project.

This is an authentic and energetic devlog about why and how I built CrawlStars. I will justify all of my design choices by sharing my thought processes for every decision I made as I was building CrawlStars. I have also integrated the <font color = "grey">~~voices in my head~~</font> most frequently asked questions into this devlog to answer the most common concerns about the project! I would appreciate any feedback you may have!

Instagram, Discord @awzheng

### Why I made CrawlStars

After playing around with Zen Browser (a popular Firefox fork) over my 1A/1B winter break I immediately realized that the URL bar autocomplete functionality was awful. I decided to take what I knew about SEO (from years of DECA case competitions) and attempted to build my own search engine while learning Go and MongoDB in the process. I've gained so much understanding and respect towards the big data and search engine industries (and would love to join them soon...)

> "Andrew! Do you play Brawl Stars?"

nah.

# Episode 1: Starting Out

## Design Choices

### Why Go over Python?

- Go is compiled. It's faster and more efficient than Python. However, the dopamine loss compared to my days of seeing my frontend update immediately when coding in Python/JS was a culture shock. lol.
- Goroutines let me spawn multiple **concurrent** workers to crawl pages at the same time. Having 10 workers makes me feel like a 10x developer while only using 10% of my CPU and several MBs of storage up in MongoDB Atlas (more on that later).
- Being statically typed means it is less prone to errors. Yes it's important since during testing, lots of websites (such as Reddit) tend to block crawlers and cause 403 errors.
- Python isn't the name of the breakout hit from CORTIS, the most notable new kpop boy band of the decade thus far.

### Why MongoDB Atlas over SQL?

The scaffolding for CrawlStars was initially built using PostgreSQL, but it was too rigid and error-prone for my purposes. Instead I pivoted to MongoDB Atlas, a NoSQL database. Here's why:

1. JSON is flexible, and my crawler extracts data that looks like json anyway (kids these days!), thus storing it in a NoSQL database is a natural fit.
2. MongoDB is optimized for high-speed ingestion which is advantageous for CrawlStars which uses 10 concurrent workers to dump data fast.
3. Atlas Search is OP. A fuzzy search engine with relevance ratings that I can scale into a rating of 0-5 stars? Built into the database??? Yes please!

## Project Diagrams

These took a while on eraser.io to make but it was totally worth learning. Also I now have a strong preference for vertical system design diagrams! (I'm feeling existential about choosing CE over SYDE...)

### Write Path
![CrawlStars Write Path](assets/CrawlStars-write-path.png)

### Read Path
![CrawlStars Read Path](assets/CrawlStars-read-path.png)

> "Andrew! Why are `Work Queue` and `Results Channel` coloured purple in your Write Path diagram? Where's the node for `crawler.go`?"

Adding the node for `Work Queue` shows that we understand **concurrency** and **transient memory** (I partly learned about RAM in my ECE courses at school, but it felt more like trying to self-learn a class that I paid for).

To illustrate what I mean, imagine if the `Extract Links` step directly triggered `Fetch HTML` with no queue. First and foremost, this would be a massive DoS attack on the website, and would most likely result in a 403 error. Even if we got through and starting "eating" the website, CrawlStars would try to crawl **all the pages** from that single URL **at the same time**. 

Remember that we created many concurrent workers for a reason (default 10), and they can each find work faster than they can finish processing it. The queue acts as a buffer to prevent us from spawning infinite goroutines and overwhelming our RAM.

Thus, `Work Queue` and `Results Channel` are purple to represent the transient memory used by `crawler.go` (as opposed to blindly running `crawler.go` infinite times before our program gets to auto-stop at 1000).



### Project Structure

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

Here are some brief descriptions of the project structure in table format:

| Directory/File | Description | Notable Member Functions |
|----------------|-------------|-------------------------|
| **cmd/** | Main entry points for the crawler and server | - |
| `cmd/crawler/main.go` | Kicks off the crawling process with 10 workers | - |
| `cmd/server/main.go` | HTTP server that handles search requests and serves the frontend | `searchHandler()`, `infoHandler()` |
| **internal/** | Core business logic for crawling and database operations | - |
| `internal/crawler/crawler.go` | Manages concurrent workers, URL queue, and recursive crawling | `New()`, `Start()`, `worker()`, `process()` |
| `internal/database/mongo.go` | Handles MongoDB connections, page storage, and Atlas Search queries | `Connect()`, `InsertPage()`, `SearchPages()` |
| **web/** | Frontend code for the search interface | - |
| `web/index.html` | Clean search UI with star ratings and clickable results | `performSearch()`, `displayResults()` |
| **go.mod** | Lists all the Go dependencies for the project | - |
| **go.sum** | Security checksums for dependencies (auto-generated by `go mod tidy`) | - |

Okay, so now that we've established what kind of project we're making (so I've established that I'm not a complete bum), let's start building!

## Episode 2: The Engine


