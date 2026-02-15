package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

const searchTimeout = 5 * time.Second

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		logger.Errorf("Failed to encode JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"internal server error"}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	params, err := parseSearchParams(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Infof("Search request received: term=%s, regexp=%s, in_path=%s", params.Term, params.Regexp, params.InPath)

	var regex *regexp.Regexp
	if params.Regexp != "" {
		regex, err = regexp.Compile(params.Regexp)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid regular expression for 'regexp'")
			logger.Errorf("Invalid regex pattern for regexp '%s': %v", params.Regexp, err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()

	result, err := performSearch(params, regex, ctx)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			writeJSONError(w, http.StatusGatewayTimeout, "Search timeout exceeded")
			logger.Errorf("Search timeout exceeded for term=%s, regexp=%s", params.Term, params.Regexp)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Error during search")
		logger.Errorf("Error during search: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matches": result.Matches,
	})

	logger.Infof("Search completed. Found %d matches for term='%s', regexp='%s', in_path='%s'",
		len(result.Matches), params.Term, params.Regexp, params.InPath)
}

func parseSearchParams(r *http.Request) (*SearchParams, error) {
	term := r.URL.Query().Get("term")
	regexpParam := r.URL.Query().Get("regexp")
	inPath := r.URL.Query().Get("in_path")
	sortOrder := r.URL.Query().Get("sort")
	showUI := r.URL.Query().Get("show_ui") == "true"

	if term == "" && regexpParam == "" && inPath == "" {
		return nil, fmt.Errorf("at least one of 'term', 'regexp', or 'in_path' query parameters is required")
	}

	if term != "" && regexpParam != "" {
		return nil, fmt.Errorf("'term' and 'regexp' are mutually exclusive, use only one")
	}

	if sortOrder != "" && sortOrder != "asc" && sortOrder != "desc" {
		return nil, fmt.Errorf("'sort' must be 'asc' or 'desc'")
	}

	return &SearchParams{
		Term:   term,
		Regexp: regexpParam,
		InPath: inPath,
		Sort:   sortOrder,
		ShowUI: showUI,
	}, nil
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	cache.RLock()
	defer cache.RUnlock()

	isRebuilding := atomic.LoadInt32(&cache.isRebuilding) == 1
	var buildDuration time.Duration
	if isRebuilding {
		buildDuration = time.Since(cache.buildStartTime)
		buildDuration = roundDurationToTenSeconds(buildDuration)
	} else {
		buildDuration = cache.buildEndTime.Sub(cache.buildStartTime)
	}

	var cacheAge time.Duration
	if !isRebuilding && !cache.buildEndTime.IsZero() {
		cacheAge = time.Since(cache.buildEndTime)
	}

	buildDurationStr := humanReadableDuration(buildDuration)
	cacheAgeStr := humanReadableDuration(cacheAge)
	cacheSizeBytes := estimateCacheSize(cache.data)
	cacheSizeHumanReadable := humanize.Bytes(cacheSizeBytes)

	totalSecrets := atomic.LoadInt64(&cache.totalSecrets)
	fetchedSecrets := atomic.LoadInt64(&cache.fetchedSecrets)
	totalKeys := atomic.LoadInt64(&cache.totalKeys)
	progress := 0
	if totalSecrets > 0 {
		progress = int(fetchedSecrets * 100 / totalSecrets)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":             version,
		"cache_age":           cacheAgeStr,
		"build_duration":      buildDurationStr,
		"is_rebuilding":       isRebuilding,
		"cache_in_mem_size":   cacheSizeHumanReadable,
		"fetched_secrets":     fetchedSecrets,
		"total_secrets":       totalSecrets,
		"total_keys_indexed":  totalKeys,
		"progress_percentage": progress,
	})

	logger.Info("Status requested")
}

func rebuildHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}

	var reqBody struct {
		Rebuild string `json:"rebuild"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body")
		logger.Errorf("Failed to decode rebuild request body: %v", err)
		return
	}

	if reqBody.Rebuild != "true" {
		writeJSONError(w, http.StatusBadRequest, "Invalid value for 'rebuild'; expected 'true'")
		return
	}

	logger.Info("Received request to rebuild cache")

	rebuildWg.Add(1)
	go func() {
		defer rebuildWg.Done()
		if err := rebuildCache(); err != nil {
			logger.Errorf("Cache rebuild failed: %v", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Cache rebuild started",
	})
}
