```eraser.io
title CrawlStars Read Path
direction right

"User" [shape: oval, icon: user, color: blue]

"web/" [color: green] {
  "index.html" [icon: layout, color: green]
  "Type Query" [shape: oval, icon: search]
  "Send Request" [shape: oval, icon: send]
  "Display Results" [shape: oval, icon: eye]
}

"cmd/server/" [color: purple] {
  "main.go" [icon: server, color: purple]
  "Receive Request" [shape: oval, icon: inbox]
  "Parse Query" [shape: oval, icon: filter]
}

"internal/database/" [color: orange] {
  "mongo.go" [icon: database, color: orange]
  "SearchPages()" [shape: oval, icon: search]
  "Calculate Stars" [shape: oval, icon: star]
}

"MongoDB Atlas" [shape: cylinder, icon: cloud, color: blue]

"User" > "index.html": "Enter search term"
"index.html" > "Type Query"
"Type Query" > "Send Request": "performSearch()"
"Send Request" > "main.go": "GET /search?q=..."
"main.go" > "Receive Request": "searchHandler()"
"Receive Request" > "Parse Query": "Get query param"
"Parse Query" > "mongo.go": "db.SearchPages()"
"mongo.go" > "SearchPages()": "Build pipeline"
"SearchPages()" > "MongoDB Atlas": "Atlas Search"
"MongoDB Atlas" > "Calculate Stars": "Return scores"
"Calculate Stars" > "Display Results": Send JSON
"Display Results" > "User": Show results
```