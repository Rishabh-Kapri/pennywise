package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func Get(url string) (any, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("error creating GET request", "url", url, "error", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		slog.Error("error executing GET request", "url", url, "error", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading GET response body", "url", url, "error", err)
		return nil, err
	}
	slog.Debug("GET response", "url", url, "body", string(body))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET request for %v failed with status code: %v", url, res.Status)
	}

	var httpRes any
	err = json.Unmarshal(body, &httpRes)
	if err != nil {
		slog.Error("error unmarshalling GET response", "url", url, "error", err)
		return nil, err
	}
	return httpRes, nil
}

func Post[T any](url string, data any) (T, error) {
	var zero T // this is the zero value for T
	requestBodyBytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("error marshaling POST request body", "url", url, "error", err)
		return zero, err
	}
	requestBody := bytes.NewBuffer(requestBodyBytes)
	req, err := http.NewRequest(http.MethodPost, url, requestBody)
	if err != nil {
		slog.Error("error creating POST request", "url", url, "error", err)
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		slog.Error("error executing POST request", "url", url, "error", err)
		return zero, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading POST response", "url", url, "error", err)
		return zero, err
	}
	// log.Printf("Response for endpoint %v: %v", url, string(body))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		slog.Error("POST request failed", "url", url, "status", res.StatusCode)
		return zero, fmt.Errorf("POST request for %v failed with status code: %v", url, res.StatusCode)
	}
	var httpRes T
	err = json.Unmarshal(body, &httpRes)
	if err != nil {
		slog.Error("error unmarshalling POST response", "url", url, "error", err)
		return zero, err
	}
	return httpRes, nil
}
