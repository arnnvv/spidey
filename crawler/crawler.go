// Package crawler
package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"spidey/database"
	"spidey/database/generated"
	"spidey/validate"
	"strings"
	"time"
)

type ModelResponse struct {
	Prediction string  `json:"prediction"`
	Confidence float64 `json:"confidence"`
}

func Crawl(ctx context.Context, targetURL string, db *database.DBService, modelApiUrl string) {
	log := slog.With("url", targetURL)
	log.Info("Starting to process URL")

	defer func() {
		if r := recover(); r != nil {
			log.Error("Panic recovered during crawl", "panic", r)
			errMsg := fmt.Sprintf("panic: %v", r)
			markAsFailed(context.Background(), db, targetURL, errMsg)
		}
	}()

	if err := db.Queries.UpdateURLStatus(ctx, generated.UpdateURLStatusParams{
		Url:    targetURL,
		Status: "classifying",
	}); err != nil {
		log.Error("Failed to update status to classifying", "error", err)
		return
	}
	log.Info("Status updated to 'classifying'")

	prediction, confidence, err := classifyURL(ctx, targetURL, modelApiUrl)
	if err != nil {
		log.Error("Failed to classify URL", "error", err)
		markAsFailed(ctx, db, targetURL, err.Error())
		return
	}
	log.Info("URL classified", "prediction", prediction, "confidence", confidence)

	if err := db.Queries.UpdateURLClassification(ctx, generated.UpdateURLClassificationParams{
		Url: targetURL,
		Classification: sql.NullString{
			String: prediction,
			Valid:  true,
		},
		Confidence: sql.NullFloat64{
			Float64: confidence,
			Valid:   true,
		},
	}); err != nil {
		log.Error("Failed to update classification in DB", "error", err)
		markAsFailed(ctx, db, targetURL, "failed to update classification")
		return
	}

	if prediction != "PERSONAL_BLOG" {
		log.Info("URL is not a personal blog, skipping crawl.")
		if err := db.Queries.MarkURLAsSkipped(ctx, targetURL); err != nil {
			log.Error("Failed to update status to skipped", "error", err)
		}
		return
	}

	if err := db.Queries.UpdateURLStatus(ctx, generated.UpdateURLStatusParams{
		Url:    targetURL,
		Status: "crawling",
	}); err != nil {
		log.Error("Failed to update status to crawling", "error", err)
		return
	}
	log.Info("Status updated to 'crawling'")

	content, links, err := fetchAndParse(ctx, targetURL)
	if err != nil {
		log.Error("Failed to fetch and parse URL", "error", err)
		markAsFailed(ctx, db, targetURL, err.Error())
		return
	}
	log.Info("Successfully fetched and parsed", "links_found", len(links))

	err = db.ExecTx(ctx, func(q *generated.Queries) error {
		if err := q.MarkURLAsCrawled(ctx, generated.MarkURLAsCrawledParams{
			Url: targetURL,
			Content: sql.NullString{
				String: content,
				Valid:  true,
			},
		}); err != nil {
			return fmt.Errorf("failed to mark URL as crawled: %w", err)
		}

		for _, link := range links {
			if validate.IsValidHTTPURL(link) {
				_, err := q.CreateURL(ctx, link)
				if err != nil {
					log.Warn("Failed to insert new link", "link", link, "error", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Error("Database transaction failed", "error", err)
		markAsFailed(ctx, db, targetURL, "database transaction failed")
		return
	}

	log.Info("Successfully crawled and stored")
}

func classifyURL(ctx context.Context, targetURL, modelApiUrl string) (string, float64, error) {
	reqBody, err := json.Marshal(map[string]string{
		"url": targetURL,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", modelApiUrl+"/predict", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create model request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("model api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("model api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var modelResp ModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode model response: %w", err)
	}

	return strings.ToUpper(modelResp.Prediction), modelResp.Confidence, nil
}

func fetchAndParse(ctx context.Context, targetURL string) (string, []string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Spidey-Crawler/1.0 (Go-lang version)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to fetch url: status %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return "", nil, fmt.Errorf("content is not HTML, but %s", contentType)
	}

	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return "", nil, fmt.Errorf("could not parse base url: %w", err)
	}

	return ExtractTextAndLinks(resp.Body, baseURL)
}

func markAsFailed(ctx context.Context, db *database.DBService, url, errMsg string) {
	if len(errMsg) > 1024 {
		errMsg = errMsg[:1024]
	}
	err := db.Queries.MarkURLAsFailed(ctx, generated.MarkURLAsFailedParams{
		Url: url,
		ErrorMessage: sql.NullString{
			String: errMsg,
			Valid:  true,
		},
	})
	if err != nil {
		slog.Error("Failed to mark URL as failed in DB", "url", url, "error", err)
	}
}
