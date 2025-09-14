package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func Get(url string) (any, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Error creating GET request for %v: %v", url, err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error GET request for %v: %v", url, err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading GET response body for %v: %v", url, err)
		return nil, err
	}
	log.Printf("GET response for %v: %v", url, string(body))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET request for %v failed with status code: %v", url, res.Status)
	}

	var httpRes any
	err = json.Unmarshal(body, &httpRes)
	if err != nil {
		log.Printf("Error unmarshalling JSON for %v: %v", url, err)
		return nil, err
	}
	return httpRes, nil
}

func Post[T any](url string, data any) (T, error) {
	var zero T // this is the zero value for T
	requestBodyBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling JSON for %v: %v", url, err)
		return zero, err
	}
	requestBody := bytes.NewBuffer(requestBodyBytes)
	req, err := http.NewRequest(http.MethodPost, url, requestBody)
	if err != nil {
		log.Printf("Error creating POST request for %v: %v", url, err)
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending POST request for %v: %v", url, err)
		return zero, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading POST response for %v: %v", url, err)
		return zero, err
	}
	// log.Printf("Response for endpoint %v: %v", url, string(body))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Printf("Error response for %v: %v", url, res.StatusCode)
		return zero, fmt.Errorf("POST request for %v failed with status code: %v", url, res.StatusCode)
	}
	var httpRes T
	err = json.Unmarshal(body, &httpRes)
	if err != nil {
		log.Printf("Error unmarshalling response for %v: %v", url, err)
		return zero, err
	}
	return httpRes, nil
}
