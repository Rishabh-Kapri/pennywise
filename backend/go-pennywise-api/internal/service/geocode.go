package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// GeocodeService resolves coordinates into a human-readable place name.
// Backed by a Nominatim-compatible endpoint (default: the public OSM one,
// which allows at most 1 request/second and requires an identifying User-Agent).
type GeocodeService interface {
	Reverse(ctx context.Context, lat float64, lng float64) (string, error)
	Search(ctx context.Context, query string, limit int) ([]PlaceResult, error)
}

// PlaceResult is one forward-geocoding hit: somewhere the user can pick.
type PlaceResult struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

const geocodeSearchMaxLimit = 8

const (
	geocodeUserAgent    = "pennywise/1.0 (self-hosted personal finance app)"
	geocodeCacheMaxSize = 512
	geocodeMinInterval  = time.Second
)

type geocodeService struct {
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	lastCall time.Time
	// cache keyed by lat/lng rounded to 4 decimals (~11m); places don't move.
	cache map[string]string
}

func NewGeocodeService(baseURL string) GeocodeService {
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org"
	}
	return &geocodeService{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cache:      map[string]string{},
	}
}

type nominatimReverseResp struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Nominatim returns coordinates as strings, not numbers.
type nominatimSearchResp struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

// throttle holds the caller until Nominatim's 1 req/s usage policy allows
// another call. Shared by Reverse and Search so the two cannot together exceed
// the limit -- the whole point of the policy is calls per client, not per method.
func (s *geocodeService) throttle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	wait := geocodeMinInterval - time.Since(s.lastCall)
	if wait > 0 {
		time.Sleep(wait)
	}
	s.lastCall = time.Now()
}

// Search resolves free text ("blue tokai indiranagar") into pickable places.
//
// Deliberately uncached: reverse lookups are cacheable because a coordinate's
// place name does not change, whereas search text is open-ended and would fill
// the cache with single-use keys.
func (s *geocodeService) Search(ctx context.Context, query string, limit int) ([]PlaceResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "search query is required")
	}
	if limit <= 0 || limit > geocodeSearchMaxLimit {
		limit = 5
	}

	s.throttle()

	reqURL := fmt.Sprintf(
		"%s/search?format=jsonv2&q=%s&limit=%d&addressdetails=0",
		s.baseURL,
		url.QueryEscape(query),
		limit,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error creating geocode search request", err)
	}
	req.Header.Set("User-Agent", geocodeUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error calling geocode search", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(errs.CodeInternalError, "geocode search returned status %d", resp.StatusCode)
	}

	var parsed []nominatimSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error decoding geocode search response", err)
	}

	results := make([]PlaceResult, 0, len(parsed))
	for _, hit := range parsed {
		lat, latErr := strconv.ParseFloat(hit.Lat, 64)
		lng, lngErr := strconv.ParseFloat(hit.Lon, 64)
		if latErr != nil || lngErr != nil {
			// Skip unusable rows rather than failing the whole search.
			continue
		}

		name := hit.DisplayName
		if hit.Name != "" && hit.DisplayName != "" {
			name = hit.Name + ", " + truncateDisplayName(hit.DisplayName, hit.Name)
		} else if hit.Name != "" {
			name = hit.Name
		}
		if name == "" {
			continue
		}

		results = append(results, PlaceResult{Name: name, Lat: lat, Lng: lng})
	}

	return results, nil
}

func (s *geocodeService) Reverse(ctx context.Context, lat float64, lng float64) (string, error) {
	key := fmt.Sprintf("%.4f,%.4f", lat, lng)

	s.mu.Lock()
	if name, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return name, nil
	}
	s.mu.Unlock()

	s.throttle()

	reqURL := fmt.Sprintf(
		"%s/reverse?format=jsonv2&lat=%s&lon=%s&zoom=18",
		s.baseURL,
		url.QueryEscape(fmt.Sprintf("%f", lat)),
		url.QueryEscape(fmt.Sprintf("%f", lng)),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "error creating geocode request", err)
	}
	req.Header.Set("User-Agent", geocodeUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "error calling geocode service", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errs.New(errs.CodeInternalError, "geocode service returned status %d", resp.StatusCode)
	}

	var parsed nominatimReverseResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "error decoding geocode response", err)
	}

	name := parsed.DisplayName
	if parsed.Name != "" {
		name = parsed.Name
		if parsed.DisplayName != "" {
			name = parsed.Name + ", " + truncateDisplayName(parsed.DisplayName, parsed.Name)
		}
	}
	if name == "" {
		return "", errs.New(errs.CodeInternalError, "geocode service returned no name")
	}

	s.mu.Lock()
	if len(s.cache) >= geocodeCacheMaxSize {
		// simple reset instead of LRU bookkeeping; cache is a courtesy only
		s.cache = map[string]string{}
	}
	s.cache[key] = name
	s.mu.Unlock()

	return name, nil
}

// truncateDisplayName keeps the first few locality parts of a Nominatim
// display_name (skipping the leading duplicate of name) so stored names stay short.
func truncateDisplayName(displayName string, name string) string {
	parts := []string{}
	for _, p := range strings.Split(displayName, ",") {
		p = strings.TrimSpace(p)
		if p == "" || p == name {
			continue
		}
		parts = append(parts, p)
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, ", ")
}
