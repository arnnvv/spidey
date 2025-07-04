// Package handler
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"spidey/crawler"
	"spidey/database"
	"spidey/validate"
)

type AddURLRequest struct {
	URL string `json:"url"`
}

type Response struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	URL     string `json:"url,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func AddURLHandler(db *database.DBService, modelApiUrl string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := slog.With("path", r.URL.Path, "method", r.Method)

		if r.Method != http.MethodPost || r.URL.Path != "/" {
			log.Warn("Not found")
			writeJSON(w, http.StatusNotFound, Response{
				Error: "Not found",
			})
			return
		}

		var reqBody AddURLRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			log.Warn("Invalid request body", "error", err)
			writeJSON(w, http.StatusBadRequest, Response{
				Error: "Invalid request body",
			})
			return
		}

		if reqBody.URL == "" {
			log.Warn("Missing URL field")
			writeJSON(w, http.StatusBadRequest, Response{
				Error: `Missing "url" field in request body`,
			})
			return
		}

		seedURL := reqBody.URL
		if !validate.IsValidHTTPURL(seedURL) {
			log.Warn("Invalid URL provided", "url", seedURL)
			writeJSON(w, http.StatusBadRequest, Response{
				Error: "Invalid URL provided",
			})
			return
		}

		_, err := db.Queries.CreateURL(r.Context(), seedURL)
		if err != nil {
			log.Error("Database insertion error", "error", err)
			writeJSON(w, http.StatusInternalServerError, Response{
				Error: "Internal Server Error",
			})
			return
		}

		crawlCtx := context.Background()

		go crawler.Crawl(crawlCtx, seedURL, db, modelApiUrl)

		log.Info("URL accepted for crawling", "url", seedURL)
		writeJSON(w, http.StatusAccepted, Response{
			Message: "URL accepted for crawling.",
			URL:     seedURL,
		})
	}
}
