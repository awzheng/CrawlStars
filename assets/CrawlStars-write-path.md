```eraser.io
title CrawlStars Write Path
direction right

"User Input" [shape: oval, icon: user, color: blue]

"cmd/crawler/" [color: green] {
  "main.go" [icon: play-circle, color: green]
  "Initialize DB" [shape: oval, icon: database]
  "Spawn Workers" [shape: oval, icon: users]
}

"internal/crawler/crawler.go" [color: purple] {
  "Work Queue" [shape: cylinder, icon: layers, color: purple]
  "Visited Check" [shape: diamond, icon: git-commit]
  "Fetch HTML" [shape: oval, icon: download]
  "Parse <body>" [shape: oval, icon: code]
  "Extract Links" [shape: oval, icon: link]
  "Results Channel" [shape: cylinder, icon: inbox, color:purple]
}

"internal/database/" [color: orange] {
  "mongo.go" [icon: database, color: orange]
  "InsertPage()" [shape: oval, icon: save]
}

"MongoDB Atlas" [shape: cylinder, icon: cloud, color: blue]

// Setup Flow
"User Input" > "main.go": "Set SEED_URL"
"main.go" > "Initialize DB": database. Connect()
"Initialize DB" > "Spawn Workers": "Start(10 workers)"
"Spawn Workers" > "Work Queue": "Push seed URL"

// Worker Loop (10 concurrent goroutines)
"Work Queue" > "Visited Check": "Worker pops URL"
"Visited Check" > "Fetch HTML": "If new URL"

// Processing
"Fetch HTML" > "Parse <body>": "Tokenize HTML"
"Parse <body>" > "Extract Links": "Find tags"
"Parse <body>" > "Results Channel": "Send page data"

// Feedback Loop
"Extract Links" > "Work Queue": "Queue new URLs"

// Storage
"Results Channel" > "mongo.go": "Read from channel"
"mongo.go" > "InsertPage()": "Upsert page"
"InsertPage()" > "MongoDB Atlas": Store page 
```