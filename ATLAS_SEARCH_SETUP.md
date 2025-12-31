# MongoDB Atlas Search Index Setup Guide

## Why Do We Need This?
CrawlStars uses **MongoDB Atlas Search** (powered by Apache Lucene) to perform fuzzy text searches across web pages. This creates an **inverted index** that maps keywords to webpages — perfect for understanding SEO!

## Step-by-Step Instructions

### 1. Log into MongoDB Atlas
Go to [MongoDB Atlas](https://cloud.mongodb.com) and select your cluster.

### 2. Navigate to Search Indexes
1. Click on your cluster name
2. Click the **"Search"** tab (not the "Indexes" tab!)
3. Click **"Create Search Index"**

### 3. Choose Atlas Search (JSON Editor)
Select **"JSON Editor"** for more control.

### 4. Configure the Index

**Database:** `crawlstars`  
**Collection:** `webpages`  
**Index Name:** `default`

**Index Definition (JSON):**
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

### 5. Create the Index
Click **"Create Search Index"** and wait 1-2 minutes for it to build.

### 6. Verify It's Active
You should see your index with status **"Active"** in green.

---

## What This Does

- **Analyzer:** `lucene.standard` 
  - Tokenizes text into words
  - Lowercases everything
  - Removes common stop words (the, a, is, etc.)
  - Creates an **inverted index** mapping words → documents

- **Fuzzy Search:** Handles typos (e.g., "computr" → "computer")

- **Relevance Scoring:** Returns a score for each match, which we normalize to 1-5 stars ⭐

---

## Testing the Index

Once the index is active, your search API will work! Try:

```bash
curl "http://localhost:8080/search?q=computer"
```

You should get JSON results with star ratings!
