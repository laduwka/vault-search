package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

var testMutex sync.Mutex
var originalCacheData map[string]*SecretKeys

func TestMain(m *testing.M) {
	originalCacheData = cache.data
	os.Exit(m.Run())
}

func restoreCache() {
	cache.Lock()
	cache.data = originalCacheData
	cache.Unlock()
}

func waitForRebuildComplete(t *testing.T, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		rebuildWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Log("Warning: rebuild did not complete within timeout")
	}
}

func TestExtractKeysFromValue(t *testing.T) {
	logEntry := logrus.NewEntry(logrus.New())

	tests := []struct {
		name     string
		data     map[string]interface{}
		expected []string
	}{
		{
			name:     "Top-level keys only",
			data:     map[string]interface{}{"username": "john", "password": "secret"},
			expected: []string{"username", "password"},
		},
		{
			name:     "Empty data",
			data:     map[string]interface{}{},
			expected: []string{},
		},
		{
			name:     "Integer and bool values",
			data:     map[string]interface{}{"port": 8080, "enabled": true},
			expected: []string{"port", "enabled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := extractKeysFromValue(tt.data, logEntry)
			if !containsAllKeys(keys, tt.expected) {
				t.Errorf("extractKeysFromValue() = %v, expected to contain %v", keys, tt.expected)
			}
		})
	}
}

func TestExtractKeysFromJSON(t *testing.T) {
	logEntry := logrus.NewEntry(logrus.New())

	tests := []struct {
		name     string
		jsonStr  string
		expected []string
	}{
		{
			name:     "Simple JSON object",
			jsonStr:  `{"host": "localhost", "port": 5432}`,
			expected: []string{"host", "port"},
		},
		{
			name:     "Nested JSON object",
			jsonStr:  `{"database": {"host": "localhost", "credentials": {"user": "admin"}}}`,
			expected: []string{"database", "host", "credentials", "user"},
		},
		{
			name:     "JSON array with objects",
			jsonStr:  `[{"name": "db1"}, {"name": "db2", "type": "postgres"}]`,
			expected: []string{"name", "type"},
		},
		{
			name:     "Invalid JSON",
			jsonStr:  `{invalid json}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			extractKeysFromJSON([]byte(tt.jsonStr), &keys, logEntry, 0)
			if !containsAllKeys(keys, tt.expected) {
				t.Errorf("extractKeysFromJSON() = %v, expected to contain %v", keys, tt.expected)
			}
		})
	}
}

func TestExtractKeysFromYAML(t *testing.T) {
	logEntry := logrus.NewEntry(logrus.New())

	tests := []struct {
		name     string
		yamlStr  string
		expected []string
	}{
		{
			name:     "Simple YAML",
			yamlStr:  "host: localhost\nport: 5432",
			expected: []string{"host", "port"},
		},
		{
			name: "Nested YAML",
			yamlStr: `database:
  host: localhost
  credentials:
    user: admin`,
			expected: []string{"database", "host", "credentials", "user"},
		},
		{
			name:     "Invalid YAML",
			yamlStr:  ":\n  : invalid",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			extractKeysFromYAML([]byte(tt.yamlStr), &keys, logEntry, 0)
			if !containsAllKeys(keys, tt.expected) {
				t.Errorf("extractKeysFromYAML() = %v, expected to contain %v", keys, tt.expected)
			}
		})
	}
}

func TestExtractNestedKeys(t *testing.T) {
	logEntry := logrus.NewEntry(logrus.New())

	tests := []struct {
		name     string
		value    interface{}
		expected []string
	}{
		{
			name:     "JSON string in value",
			value:    `{"api_key": "abc123", "endpoint": "/v1"}`,
			expected: []string{"api_key", "endpoint"},
		},
		{
			name:     "YAML string in value",
			value:    "server:\n  host: localhost\n  port: 8080",
			expected: []string{"server", "host", "port"},
		},
		{
			name:     "Plain string (no extraction)",
			value:    "just a regular string",
			expected: []string{},
		},
		{
			name:     "Map value",
			value:    map[string]interface{}{"nested": map[string]interface{}{"key": "value"}},
			expected: []string{"nested", "key"},
		},
		{
			name:     "Array with maps",
			value:    []interface{}{map[string]interface{}{"name": "item1"}, map[string]interface{}{"type": "A"}},
			expected: []string{"name", "type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			extractNestedKeys(tt.value, &keys, logEntry, 0)
			if !containsAllKeys(keys, tt.expected) {
				t.Errorf("extractNestedKeys() = %v, expected to contain %v", keys, tt.expected)
			}
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`{"key": "value"}`, true},
		{`[1, 2, 3]`, true},
		{`  {"spaced": true}`, true},
		{`plain text`, false},
		{`key: value`, false},
	}

	for _, tt := range tests {
		result := looksLikeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("looksLikeJSON(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestLooksLikeYAML(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"host: localhost\nport: 5432", true},
		{"single: line", false},
		{"no colons\njust text", false},
		{"key: value\nanother: item", true},
	}

	for _, tt := range tests {
		result := looksLikeYAML(tt.input)
		if result != tt.expected {
			t.Errorf("looksLikeYAML(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseSearchParams(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
		params      *SearchParams
	}{
		{
			name:        "Valid term search",
			url:         "/search?term=password",
			expectError: false,
			params:      &SearchParams{Term: "password"},
		},
		{
			name:        "Valid regexp search",
			url:         "/search?regexp=^pass",
			expectError: false,
			params:      &SearchParams{Regexp: "^pass"},
		},
		{
			name:        "Valid in_path search",
			url:         "/search?in_path=prod",
			expectError: false,
			params:      &SearchParams{InPath: "prod"},
		},
		{
			name:        "Missing all params",
			url:         "/search",
			expectError: true,
			errorMsg:    "at least one of 'term', 'regexp', or 'in_path'",
		},
		{
			name:        "Both term and regexp",
			url:         "/search?term=pass&regexp=^pass",
			expectError: true,
			errorMsg:    "'term' and 'regexp' are mutually exclusive",
		},
		{
			name:        "Invalid sort value",
			url:         "/search?term=pass&sort=invalid",
			expectError: true,
			errorMsg:    "'sort' must be 'asc' or 'desc'",
		},
		{
			name:        "Valid with all options",
			url:         "/search?term=pass&in_path=prod&sort=asc&show_ui=true",
			expectError: false,
			params:      &SearchParams{Term: "pass", InPath: "prod", Sort: "asc", ShowUI: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			params, err := parseSearchParams(req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Error message = %v, expected to contain %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if params.Term != tt.params.Term || params.Regexp != tt.params.Regexp ||
					params.InPath != tt.params.InPath || params.Sort != tt.params.Sort ||
					params.ShowUI != tt.params.ShowUI {
					t.Errorf("Params = %v, expected %v", params, tt.params)
				}
			}
		})
	}
}

func TestMatchSecret(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		keys     *SecretKeys
		params   *SearchParams
		expected bool
	}{
		{
			name:     "Term match - case insensitive",
			path:     "prod/db/credentials",
			keys:     &SecretKeys{SearchString: "prod/db/credentials username password "},
			params:   &SearchParams{Term: "PASSWORD"},
			expected: true,
		},
		{
			name:     "Term no match",
			path:     "prod/db/credentials",
			keys:     &SecretKeys{SearchString: "prod/db/credentials username "},
			params:   &SearchParams{Term: "api_key"},
			expected: false,
		},
		{
			name:     "Regexp match",
			path:     "prod/api/keys",
			keys:     &SecretKeys{SearchString: "prod/api/keys api_key secret_key "},
			params:   &SearchParams{Regexp: "(?i)api_key"},
			expected: true,
		},
		{
			name:     "Regexp case sensitive no match",
			path:     "prod/db/config",
			keys:     &SecretKeys{SearchString: "prod/db/config password "},
			params:   &SearchParams{Regexp: "^PASS"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var regex *regexp.Regexp
			var err error
			if tt.params.Regexp != "" {
				regex, err = regexp.Compile(tt.params.Regexp)
				if err != nil {
					t.Fatalf("Invalid regexp: %v", err)
				}
			}

			result := matchSecret(tt.path, tt.keys, tt.params, regex)
			if result != tt.expected {
				t.Errorf("matchSecret() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDetermineMatches(t *testing.T) {
	contentMatches := []string{"prod/db/creds", "prod/api/keys", "staging/db/config"}
	pathMatches := []string{"prod/db/creds", "prod/api/keys", "prod/cache"}

	tests := []struct {
		name          string
		params        *SearchParams
		expectedCount int
		expectedIn    []string
		expectedNotIn []string
	}{
		{
			name:          "Content search only",
			params:        &SearchParams{Term: "password"},
			expectedCount: 3,
			expectedIn:    contentMatches,
		},
		{
			name:          "Path search only",
			params:        &SearchParams{InPath: "prod"},
			expectedCount: 3,
			expectedIn:    pathMatches,
		},
		{
			name:          "Combined - intersection",
			params:        &SearchParams{Term: "pass", InPath: "prod"},
			expectedCount: 2,
			expectedIn:    []string{"prod/db/creds", "prod/api/keys"},
			expectedNotIn: []string{"staging/db/config", "prod/cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := determineMatches(tt.params, contentMatches, pathMatches)
			if len(matches) != tt.expectedCount {
				t.Errorf("determineMatches() count = %d, expected %d", len(matches), tt.expectedCount)
			}
			for _, expected := range tt.expectedIn {
				if !containsString(matches, expected) {
					t.Errorf("Expected %s in matches but not found", expected)
				}
			}
			for _, notExpected := range tt.expectedNotIn {
				if containsString(matches, notExpected) {
					t.Errorf("Did not expect %s in matches but found", notExpected)
				}
			}
		})
	}
}

func TestMatchInPath(t *testing.T) {
	tests := []struct {
		name       string
		secretPath string
		inPath     string
		expected   bool
	}{
		{"exact match", "prod", "prod", true},
		{"starts with segment", "prod/db/creds", "prod", true},
		{"middle segment", "staging/prod/db", "prod", true},
		{"ends with segment", "staging/prod", "prod", true},
		{"substring in word - no match", "game-products/rabbitmq", "prod", false},
		{"prefix of segment - no match", "production/db", "prod", false},
		{"suffix of segment - no match", "staging/hotprod/db", "prod", false},
		{"multi-segment inPath", "staging/prod/db/creds", "prod/db", true},
		{"multi-segment at start", "prod/db/creds", "prod/db", true},
		{"no match at all", "staging/dev/config", "prod", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchInPath(tt.secretPath, tt.inPath)
			if result != tt.expected {
				t.Errorf("matchInPath(%q, %q) = %v, expected %v", tt.secretPath, tt.inPath, result, tt.expected)
			}
		})
	}
}

func TestBuildSearchString(t *testing.T) {
	path := "prod/db/credentials"
	keys := []string{"username", "password", "host"}

	result := buildSearchString(path, keys)

	if !strings.Contains(result, "prod/db/credentials") {
		t.Error("Search string should contain path")
	}
	if !strings.Contains(result, "username") {
		t.Error("Search string should contain key 'username'")
	}
	if !strings.Contains(result, "password") {
		t.Error("Search string should contain key 'password'")
	}
	if result != strings.ToLower(result) {
		t.Error("Search string should be lowercase")
	}
}

func TestSearchHandler(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	defer restoreCache()
	setupTestCache()

	tests := []struct {
		name         string
		url          string
		expectStatus int
	}{
		{
			name:         "Valid term search",
			url:          "/search?term=password",
			expectStatus: http.StatusOK,
		},
		{
			name:         "Missing params",
			url:          "/search",
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Both term and regexp",
			url:          "/search?term=pass&regexp=^pass",
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Invalid regexp",
			url:          "/search?regexp=[invalid(",
			expectStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			searchHandler(rec, req)

			if rec.Code != tt.expectStatus {
				t.Errorf("Status = %d, expected %d", rec.Code, tt.expectStatus)
			}
		})
	}
}

func TestSearchHandlerWithSort(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	defer restoreCache()
	setupTestCache()

	req := httptest.NewRequest(http.MethodGet, "/search?term=key&sort=asc", nil)
	rec := httptest.NewRecorder()

	searchHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, expected %d", rec.Code, http.StatusOK)
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	matches := response["matches"].([]interface{})
	for i := 1; i < len(matches); i++ {
		if matches[i-1].(string) > matches[i].(string) {
			t.Error("Results not sorted in ascending order")
		}
	}
}

func TestSearchHandlerShowUI(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	defer restoreCache()
	setupTestCache()

	req := httptest.NewRequest(http.MethodGet, "/search?term=password&show_ui=true", nil)
	rec := httptest.NewRecorder()

	searchHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, expected %d", rec.Code, http.StatusOK)
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	matches := response["matches"].([]interface{})
	if len(matches) > 0 {
		url := matches[0].(string)
		if !strings.Contains(url, "/ui/vault/secrets/") {
			t.Errorf("Expected Vault UI URL, got %s", url)
		}
	}
}

func TestRebuildHandler(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	tests := []struct {
		name         string
		method       string
		body         string
		expectStatus int
		waitForAsync bool
	}{
		{
			name:         "Wrong method",
			method:       http.MethodGet,
			body:         ``,
			expectStatus: http.StatusMethodNotAllowed,
			waitForAsync: false,
		},
		{
			name:         "Invalid JSON",
			method:       http.MethodPost,
			body:         `{invalid}`,
			expectStatus: http.StatusBadRequest,
			waitForAsync: false,
		},
		{
			name:         "Wrong rebuild value",
			method:       http.MethodPost,
			body:         `{"rebuild": "false"}`,
			expectStatus: http.StatusBadRequest,
			waitForAsync: false,
		},
		{
			name:         "Valid rebuild request",
			method:       http.MethodPost,
			body:         `{"rebuild": "true"}`,
			expectStatus: http.StatusOK,
			waitForAsync: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/rebuild", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			rebuildHandler(rec, req)

			if rec.Code != tt.expectStatus {
				t.Errorf("Status = %d, expected %d", rec.Code, tt.expectStatus)
			}

			if tt.waitForAsync {
				waitForRebuildComplete(t, 2*time.Second)
			}
		})
	}
}

func TestHumanReadableDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m 1s"},
		{90061 * time.Second, "1d 1h 1m 1s"},
	}

	for _, tt := range tests {
		result := humanReadableDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("humanReadableDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

func TestRoundDurationToTenSeconds(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected time.Duration
	}{
		{5 * time.Second, 0},
		{12 * time.Second, 10 * time.Second},
		{15 * time.Second, 10 * time.Second},
		{18 * time.Second, 10 * time.Second},
		{25 * time.Second, 20 * time.Second},
	}

	for _, tt := range tests {
		result := roundDurationToTenSeconds(tt.duration)
		if result != tt.expected {
			t.Errorf("roundDurationToTenSeconds(%v) = %v, expected %v", tt.duration, result, tt.expected)
		}
	}
}

func TestIsPermissionDenied(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{createError("permission denied"), true},
		{createError("403 Forbidden"), true},
		{createError("connection refused"), false},
		{createError("not found"), false},
	}

	for _, tt := range tests {
		result := isPermissionDenied(tt.err)
		if result != tt.expected {
			t.Errorf("isPermissionDenied(%v) = %v, expected %v", tt.err, result, tt.expected)
		}
	}
}

func TestPerformSearchTimeout(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	defer restoreCache()
	setupLargeTestCache()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	params := &SearchParams{Term: "test"}
	_, err := performSearch(params, nil, ctx)

	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestPerformSearch(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	defer restoreCache()
	setupTestCache()

	tests := []struct {
		name          string
		params        *SearchParams
		expectedCount int
	}{
		{
			name:          "Search by term",
			params:        &SearchParams{Term: "password"},
			expectedCount: 2,
		},
		{
			name:          "Search by path",
			params:        &SearchParams{InPath: "prod"},
			expectedCount: 2,
		},
		{
			name:          "Combined search",
			params:        &SearchParams{Term: "api", InPath: "prod"},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := performSearch(tt.params, nil, ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(result.Matches) != tt.expectedCount {
				t.Errorf("performSearch() count = %d, expected %d", len(result.Matches), tt.expectedCount)
			}
		})
	}
}

func setupTestCache() {
	cache.Lock()
	cache.data = map[string]*SecretKeys{
		"prod/db/credentials": {
			AllKeys:      []string{"username", "password", "host"},
			SearchString: "prod/db/credentials username password host ",
		},
		"prod/api/keys": {
			AllKeys:      []string{"api_key", "secret_key"},
			SearchString: "prod/api/keys api_key secret_key ",
		},
		"staging/db/config": {
			AllKeys:      []string{"host", "port", "password"},
			SearchString: "staging/db/config host port password ",
		},
	}
	cache.Unlock()
}

func setupLargeTestCache() {
	data := make(map[string]*SecretKeys)
	for i := 0; i < 10000; i++ {
		path := "path/" + string(rune(i))
		data[path] = &SecretKeys{
			AllKeys:      []string{"key1", "key2"},
			SearchString: path + " key1 key2 ",
		}
	}
	cache.Lock()
	cache.data = data
	cache.Unlock()
}

func containsAllKeys(keys []string, expected []string) bool {
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, e := range expected {
		if !keySet[e] {
			return false
		}
	}
	return true
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func createError(msg string) error {
	return &testError{msg: msg}
}
