package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// httpScraper implements comic.ScraperClient by POSTing to the Python scraper
// service (SPEC-02 P1.8). The scraper reads its own callback URL + shared secret +
// S3 creds from env; this only passes the per-scrape parameters and returns as soon
// as the job is accepted (the scraper works async and calls the sync-callback).
type httpScraper struct {
	baseURL string
	client  *http.Client
}

func newHTTPScraper(baseURL string) *httpScraper {
	return &httpScraper{baseURL: baseURL, client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *httpScraper) StartScrape(ctx context.Context, sourceID uuid.UUID, sourceURL, chaptersHint string, existing []string) error {
	body, _ := json.Marshal(map[string]string{
		"source_id":  sourceID.String(),
		"source_url": sourceURL,
		"chapters":   chaptersHint,
		"existing":   strings.Join(existing, ","),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/sync", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("scraper %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *httpScraper) CancelScrape(ctx context.Context, sourceID uuid.UUID) error {
	body, _ := json.Marshal(map[string]string{"source_id": sourceID.String()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/cancel", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
