package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func newAuthenticatedRequest(method, rawURL string, body io.Reader, session sessionData) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	return req, nil
}

func readAPIError(resp *http.Response, fallback string) error {
	body, _ := io.ReadAll(resp.Body)
	var apiErr struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
		return errors.New(apiErr.Error)
	}
	if fallback != "" {
		return errors.New(fallback)
	}
	return errors.New(resp.Status)
}

func runGameCreate(ctx context.Context, file string, rt *drynnRuntime) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	if rt.session.AccessToken == "" {
		return fmt.Errorf("not logged in; run 'drynn login' first")
	}

	body, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if !json.Valid(body) {
		return fmt.Errorf("file %q does not contain valid JSON", file)
	}

	endpoint, err := url.JoinPath(rt.serverURL, "/api/v1/games")
	if err != nil {
		return fmt.Errorf("build games URL: %w", err)
	}

	req, err := newAuthenticatedRequest(http.MethodPost, endpoint, bytes.NewReader(body), rt.session)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create game request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return readAPIError(resp, fmt.Sprintf("create game failed: %s", resp.Status))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	fmt.Println(string(respBody))
	return nil
}
