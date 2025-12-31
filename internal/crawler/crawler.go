package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andrewzheng/CrawlStars/internal/database"
	"golang.org/x/net/html"
)

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

// New constructor for a new Crawler.
func New(db *database.DB) *Crawler {
	return &Crawler{
		db:        db,
		Queue:     make(chan string, 1000),       // Buffer the queue.
		Results:   make(chan database.Page, 100), // Buffer the results.
		maxCrawls: 1000,                          // Default limit: 1000 pages
	}
}

// Start unleashes the crawler.
// seedUrl: where to start.
// workers: how many minions to spawn.
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

func (c *Crawler) processResults(wg *sync.WaitGroup) {
	defer wg.Done()
	for page := range c.Results {
		err := c.db.InsertPage(page)
		if err != nil {
			c.failedCount.Add(1)
			fmt.Printf("💩 Failed to save page %s: %v\n", page.URL, err)
		} else {
			c.savedCount.Add(1)
		}
	}
}

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

// isVisited checks the memory bank.
func (c *Crawler) isVisited(u string) bool {
	_, ok := c.visited.Load(u)
	return ok
}

// markVisited stamps the passport.
func (c *Crawler) markVisited(u string) {
	c.visited.Store(u, true)
}

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

// extractTitle creates a fallback title from content if no <title> tag was found.
func extractTitle(content string) string {
	if len(content) > 50 {
		runes := []rune(content)
		if len(runes) > 50 {
			return string(runes[:50]) + "..."
		}
	}
	return content
}

// printStats shows the final crawl statistics.
func (c *Crawler) printStats() {
	duration := time.Since(c.startTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CRAWL COMPLETE! Here's what went down:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Duration:          %s\n", duration.Round(time.Second))
	fmt.Printf("Total Attempted:   %d pages\n", c.crawledCount.Load())
	fmt.Printf("Successfully Saved: %d pages\n", c.savedCount.Load())
	fmt.Printf("Failed:            %d pages\n", c.failedCount.Load())

	if duration.Seconds() > 0 {
		rate := float64(c.savedCount.Load()) / duration.Seconds()
		fmt.Printf("Crawl Speed:       %.2f pages/second\n", rate)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✨ Data is now searchable via the server API!")
	fmt.Println(strings.Repeat("=", 60) + "\n")
}
