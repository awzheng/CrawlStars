# CrawlStars DevLog

> Never have I ever... felt so passionate before about a database project.

This is an authentic and energetic devlog about why and how I built CrawlStars. 
I will justify all of my design choices by sharing my thought processes for every decision I made as I was building CrawlStars. 
I have also integrated the <font color = "grey">~~voices in my head~~</font> most frequently asked questions into this devlog to answer the most common concerns about the project! 
I would appreciate any feedback you may have!

Instagram, Discord @awzheng

### Why I made CrawlStars

After playing around with Zen Browser (a popular Firefox fork) over my 1A/1B winter break I immediately realized that the URL bar autocomplete functionality was awful. 
I decided to take what I knew about SEO (from years of DECA case competitions) and attempted to build my own search engine while learning Go and MongoDB in the process. 
I've gained so much understanding and respect towards the big data and search engine industries (and would love to join them soon...)

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

The scaffolding for CrawlStars was initially built using PostgreSQL, but it was too rigid and error-prone for my purposes. 
Instead I pivoted to MongoDB Atlas, a NoSQL database. Here's why:

1. JSON is flexible, and my crawler extracts data that looks like json anyway (kids these days!), thus storing it in a NoSQL database is a natural fit.
2. MongoDB is optimized for high-speed ingestion which is advantageous for CrawlStars which uses 10 concurrent workers to dump data fast.
3. Atlas Search is OP. A fuzzy search engine with relevance ratings that I can scale into a rating of 0-5 stars? Built into the database??? Yes please!

## Project Diagrams

These took a while on eraser.io to make but it was totally worth learning. 
Also I now have a strong preference for vertical system design diagrams! (I'm feeling existential about choosing CE over SYDE...)

### Write Path
![CrawlStars Write Path](assets/CrawlStars-write-path.png)

### Read Path
![CrawlStars Read Path](assets/CrawlStars-read-path.png)

> "Andrew! Why are `Work Queue` and `Results Channel` coloured purple in your Write Path diagram? Where's the node for `crawler.go`?"

Adding the node for `Work Queue` shows that we understand **concurrency** and **transient memory** (I partly learned about RAM in my ECE courses at school, but it felt more like trying to self-learn a class that I paid for).

To illustrate what I mean, imagine if the `Extract Links` step directly triggered `Fetch HTML` with no queue. 
First and foremost, this would be a massive DoS attack on the website, and would most likely result in a 403 error. 
Even if we got through and starting "eating" the website, CrawlStars would try to crawl **all the pages** from that single URL **at the same time**. 

Remember that we created many concurrent workers for a reason (default 10), and they can each find work faster than they can finish processing it. 
The queue acts as a buffer to prevent us from spawning infinite goroutines and overwhelming our RAM.

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

Okay, so now that we've established what kind of project we're making (and I'm not a complete bum), let's start building!

# Episode 2: The Crawlers

Now that we've laid out the project structure, it's time to make these parts move.
I'll be breaking down the logic and decisions behind the code.
Get ready for 300 lines of enlightenment.

## internal/crawler/crawler.go

The flagship of our project, `crawler.go` manages our concurrent workers (crawlers) and our work queue (work queue).

### Crawler Struct

Here's the struct of `crawler.go`. A struct is like a blueprint for an object, and we can create multiple objects from the same struct.

```go
// Crawler is the hungry beast that eats the web.
type Crawler struct {
	db            *database.DB
	Queue         chan string
	Results       chan database.Page
	visited       sync.Map
	wg            sync.WaitGroup
	crawledCount  atomic.Int64 // Thread-safe counter for total pages crawled
	savedCount    atomic.Int64 // Thread-safe counter for successfully saved pages
	failedCount   atomic.Int64 // Thread-safe counter for failed pages
	maxCrawls     int64        // Stop after this many successful crawls
	stopRequested atomic.Bool  // Signal to stop all workers
	startTime     time.Time    // When we started crawling
}
```

A few notable features of the struct:
- visited URLs are saved to the `visited` sync.Map (bool) which ensures no duplicate URLs are crawled.
- I made several helper functions such as `isVisited()` and `markVisited()` for checking the map, and `extractTitle()` as a fallback title for URLs without `<title>` tags.
- wg is a WaitGroup (a tracker of how many workers are still running) which is used to wait for all workers to finish before the crawler can stop.

> Andrew! What's atomic.Int64?

Atomic types ensure thread safety by using atomic operations: indivisible and can't be interrupted by other threads. It uses low-level CPU instructions to update the values instantly. Seemed very appropriate for our crawler purposes!

### Constructor

Here's the constructor for `crawler.go`. 
The constructor is called to create a Crawler instance. Then, the workers are spawned in `Start()`. In simple terms:
- `New()` = build the factory
- `Start()` = spawn the minions

```go
// New constructor for a new Crawler.
func New(db *database.DB) *Crawler {
	return &Crawler{
		db:        db,
		Queue:     make(chan string, 1000),       // Buffer the queue.
		Results:   make(chan database.Page, 100), // Buffer the results.
		maxCrawls: 1000,                          // Default limit: 1000 pages
	}
}
```

I decided to set the default limit of max crawls to 1000.
Initially it was 5000, but that took several minutes per test and a lot of the pages explored a few thousand crawls in were only slight variants of other previously visited URLs.

### Start()

The `Start()` function initializes everything and gets the workers moving.

```go
func (c *Crawler) Start(seedUrl string, workers int) {
	c.startTime = time.Now()
	fmt.Println("Liftoff! Starting crawl at:", seedUrl)
	fmt.Printf("Workers: %d | Max Crawls: %d\n\n", workers, c.maxCrawls)

	// Feed the beast.
	c.Queue <- seedUrl

	// Start the Results processor (DB Inserter) in a background routine.
	var processorWg sync.WaitGroup
	processorWg.Add(1)
	go c.processResults(&processorWg)

	// Spawn the minions.
	for i := 0; i < workers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}

	// Wait for the minions to finish.
	c.wg.Wait()

	// Close result channel after workers are done
	close(c.Results)

	// Wait for processor to finish saving everything
	processorWg.Wait()

	// Show the final stats
	c.printStats()
}
```

Here's what happens step by step:

1. **Record start time**: We timestamp when crawling begins to calculate crawl speed later.
2. **Push seed URL**: SEED_URL is the first URL we crawl. It gets added to the `Queue` channel.
3. **Spawn workers**: In the for loop, we create `workers` number of goroutines (default 10), each running the `worker()` function concurrently.
4. **Background processor**: A separate goroutine (not a minion!) runs `processResults()` to save pages to the database.
5. **Wait for completion**: Once our workers finish (either by reaching 1000 or running out of URLs), `wg.Wait()` unblocks.
6. **Clean up**: Close both channels and print final statistics. At this point we're ready to run the search engine and see what we've crawled.

> "Andrew! Why do we need a separate `processResults()` goroutine? Can't workers just save to the database directly?"

This is a design pattern I learned called **producer-consumer**. 

- Picture this: if each worker saved directly to MongoDB, we'd have 10 concurrent database connections fighting for resources. 
- Instead, workers produce pages to the `Results` channel, and the consumer goroutine processes them sequentially. 
- This way, the crawling speed is seperate from the saving speed. Even if Mongo gets slow, it won't stop our workers from crawling.
- So we have a producer worker (crawler) and a consumer worker (processor).
- This makes me feel like an alpha leader and also keeps my bank account sane. 
- No need to thank me! Banks hire me pls

### worker()

There's a lot to unpack here. Brace yourself:

```go
// worker is a minion that endlessly consumes URLs.
func (c *Crawler) worker(id int) {
	defer c.wg.Done()

	for {
		// Check if we hit the limit
		if c.stopRequested.Load() {
			fmt.Printf("[Worker %d] 🛑 Stop signal received. Shutting down.\n", id)
			return
		}

		select {
		case u := <-c.Queue:
			if c.isVisited(u) {
				continue
			}
			c.markVisited(u)

			// Increment counter BEFORE processing (so we know which # this is)
			count := c.crawledCount.Add(1)

			// Check if we've hit the limit
			if count > c.maxCrawls {
				fmt.Printf("[Worker %d] 🎯 Reached max crawls (%d). Initiating shutdown.\n", id, c.maxCrawls)
				c.stopRequested.Store(true)
				return
			}

			content, title, links, err := c.process(u)
			if err != nil {
				c.failedCount.Add(1)
				fmt.Printf("Count: %d | [Worker %d] ❌ Indigestion: %s | %v\n", count, id, u, err)
				continue
			}

			// Success! Print with title
			fmt.Printf("Count: %d | [Worker %d] ✅ Eating: %s | \"%s\"\n", count, id, u, title)

			// Send to Results channel for processing
			c.Results <- database.Page{
				URL:     u,
				Title:   title,
				Content: content,
			}

			// Add new links to the menu (if we haven't hit limit yet)
			if c.stopRequested.Load() {
				return
			}

			for _, link := range links {
				if !c.isVisited(link) {
					// Non-blocking send
					select {
					case c.Queue <- link:
					default:
						// Queue full, skip it
					}
				}
			}

		case <-time.After(5 * time.Second):
			// Timeout if queue is empty
			fmt.Printf("[Worker %d] 💤 Queue empty for 5s. Quitting.\n", id)
			return
		}
	}
}
```

Our worker is a never-ending loop (empty `for` loop) that consumes URLs from the queue and processes them.
Once the URL is processed, the worker spits the extracted links from that URL back to the queue for processing and repeats the process for those links. The original (now processed) URL gets sent to the `Results` channel for database storage.

This is where the purple mini-loop **Write Path** diagram occurs. If we follow the flow of a URL through the system here's what we get:

1. **Work Queue → Visited Check**: Worker pulls URL from `Queue`, checks `isVisited()` (the diamond represents a decision point)
2. **If visited**: Skip to next URL (not pictured in the diagram for cleanliness, but it just loops back to the Work Queue)
3. **If new**: Mark as visited, then proceed to `process(url)`
4. **Fetch HTML → Parse Body → Extract Links**: All handled inside `process()` (more on this later)
5. **Extract Links → Work Queue**: Discovered links get pushed back to `Queue` (which is our main feedback loop)
6. **Parse Body → Results Channel**: Page data also goes to `Results` channel for database insertion through `mongo.go`

> "Andrew! What happens if the queue is full?"

The `select` statement with `default` is a non-blocking send, so it won't wait around for the queue to become available before moving on. 
If the queue is full (1000 URLs buffered), we skip that link instead.
Otherwise, the worker would block and wait for a slot to open, ruining our concurrency.
Thus, the main tradeoff we're making here is favouring throughput over completeness.
After all, we have 10 workers and they got work to do!
- Like some random guy in the Waterloo plaza once said: you can't change the girl, you gotta change the girl.
- don't worry that one took me a while too.

### process()

This is where we actually fetch and parse the HTML, spotted in the `worker()` as

```go
content, title, links, err := c.process(u)
```

where we process and extract the contents of the URL, essentially the "fetch and parse" part of the diagram (between `Visited Check` and `Results Channel`).
Warning: the following code is egregiously long, so prepare your scrolling fingers.
Personally I'm a middle-and-ring scroller. I feel like a rockstar when I scroll.

```go
// process fetches and parses the URL. Returns content, title, links, and error.
func (c *Crawler) process(u string) (content string, title string, links []string, err error) {
	// 1. Fetch (with manners - sites like Wikipedia hate rude bots)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", "", nil, err
	}

	// Put on our fancy disguise. Without a User-Agent, websites think we're a spam bot.
	req.Header.Set("User-Agent", "CrawlStars/1.0 (+https://github.com/andrewzheng/CrawlStars; Educational Crawler)")

	client := &http.Client{
		Timeout: 15 * time.Second, // Don't wait forever for slow sites.
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	// 2. Parse (Tokenize)
	z := html.NewTokenizer(resp.Body)
	var contentBuilder strings.Builder
	var titleText string
	inBody := false
	inTitle := false
	charCount := 0 // Count characters, not tokens (per user request)

	for {
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				// Use title if found, otherwise use first 50 chars of content
				if titleText == "" {
					titleText = extractTitle(contentBuilder.String())
				}
				return contentBuilder.String(), titleText, links, nil
			}
			return "", "", nil, z.Err()

		case html.StartTagToken, html.EndTagToken:
			t := z.Token()

			// Track when we're inside <body>
			if t.Data == "body" {
				inBody = (tt == html.StartTagToken)
			}

			// Track when we're inside <title> to extract page title
			if t.Data == "title" {
				inTitle = (tt == html.StartTagToken)
			}

			// Extract links from <a> tags
			if tt == html.StartTagToken && t.Data == "a" {
				for _, a := range t.Attr {
					if a.Key == "href" {
						parsedLink, err := url.Parse(a.Val)
						if err == nil {
							base, _ := url.Parse(u)
							resolved := base.ResolveReference(parsedLink)
							if resolved.Scheme == "http" || resolved.Scheme == "https" {
								links = append(links, resolved.String())
							}
						}
					}
				}
			}

		case html.TextToken:
			text := strings.TrimSpace(z.Token().Data)
			if len(text) == 0 {
				continue
			}

			// Extract title text from <title> tag
			if inTitle && titleText == "" {
				titleText = text
			}

			// Extract body content (first 500 characters only)
			if inBody && charCount < 500 {
				// Filter out very short fragments (likely navigation/menu items)
				// Only accept text that's at least 10 characters to avoid noise
				if len(text) < 10 {
					continue
				}

				remaining := 500 - charCount
				if len(text) > remaining {
					contentBuilder.WriteString(text[:remaining])
					charCount = 500 // We're done collecting content
				} else {
					contentBuilder.WriteString(text + " ")
					charCount += len(text) + 1
				}
			}
		}
	}
}
```

Here's a quick summary.

1. **User-Agent header**: Without this, many sites block us with 403 Forbidden errors. We identify ourselves as an educational crawler because that's who we are. Educational. And hopefully an employed one at that.
2. **15-second timeout**: Prevents loitering on slow sites, I feel too slow for this life as it is, let's not make things worse.
3. **html.Tokenizer**: We use Go's HTML tokenizer to extract:
   - Page title from `<title>` tag
   - First 500 characters from `<body>` (filtering out fragments < 10 chars to avoid navigation noise)
   - All `<a>` tag links for the feedback loop

...and it can be simplified into the clean nodes `Fetch HTML → Parse <body> → Extract Links` that you see in the diagram. Impressive huh?




