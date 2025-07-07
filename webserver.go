package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

var indexTemplate = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>SubTrends Web</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 2em; }
        textarea { width: 100%; height: 400px; }
    </style>
</head>
<body>
    <h1>SubTrends Web</h1>
    <form method="POST" action="/summarize">
        <input type="text" name="subreddit" placeholder="Enter subreddit" required>
        <button type="submit">Summarize</button>
    </form>
    {{if .Summary}}
    <h2>Summary</h2>
    <pre>{{.Summary}}</pre>
    {{end}}
    {{if .Error}}
    <p style="color:red;">{{.Error}}</p>
    {{end}}
</body>
</html>
`))

func indexHandler(w http.ResponseWriter, r *http.Request) {
	indexTemplate.Execute(w, nil)
}

func summarizeHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}
	subreddit := r.FormValue("subreddit")
	token, err := getRedditAccessToken()
	if err != nil {
		renderPage(w, "", fmt.Sprintf("error getting token: %v", err))
		return
	}
	data, err := subredditData(subreddit, token)
	if err != nil {
		renderPage(w, "", fmt.Sprintf("error fetching data: %v", err))
		return
	}
	summary, err := summarizePosts(data)
	if err != nil {
		renderPage(w, "", fmt.Sprintf("error summarizing: %v", err))
		return
	}
	renderPage(w, summary, "")
}

func renderPage(w http.ResponseWriter, summary, errMsg string) {
	indexTemplate.Execute(w, struct {
		Summary string
		Error   string
	}{Summary: summary, Error: errMsg})
}

func startWebServer() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/summarize", summarizeHandler)
	log.Println("Starting web server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
