package main

import (
	"encoding/json"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const maxNestedDepth = 10

func extractKeysFromValue(data map[string]interface{}, logEntry *logrus.Entry) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
		value := data[key]
		extractNestedKeys(value, &keys, logEntry.WithField("parent_key", key), 0)
	}
	return keys
}

func extractNestedKeys(value interface{}, keys *[]string, logEntry *logrus.Entry, depth int) {
	if depth >= maxNestedDepth {
		return
	}
	switch v := value.(type) {
	case string:
		if looksLikeJSON(v) {
			extractKeysFromJSON([]byte(v), keys, logEntry, depth+1)
		} else if looksLikeYAML(v) {
			extractKeysFromYAML([]byte(v), keys, logEntry, depth+1)
		}
	case map[string]interface{}:
		collectKeysFromMap(v, keys, logEntry, depth+1)
	case []interface{}:
		for i, item := range v {
			extractNestedKeys(item, keys, logEntry.WithField("array_index", i), depth+1)
		}
	}
}

func looksLikeJSON(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func looksLikeYAML(s string) bool {
	return strings.Contains(s, ":") && strings.Contains(s, "\n")
}

func extractKeysFromJSON(data []byte, keys *[]string, logEntry *logrus.Entry, depth int) {
	logEntry.Debug("Detected potential JSON in value, attempting to parse")

	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		logEntry.WithError(err).Debug("Failed to parse JSON in value, skipping nested key extraction")
		return
	}

	switch v := parsed.(type) {
	case map[string]interface{}:
		collectKeysFromMap(v, keys, logEntry, depth)
	case []interface{}:
		for i, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				collectKeysFromMap(m, keys, logEntry.WithField("array_index", i), depth)
			}
		}
	}
}

func extractKeysFromYAML(data []byte, keys *[]string, logEntry *logrus.Entry, depth int) {
	logEntry.Debug("Detected potential YAML in value, attempting to parse")

	var parsed interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		logEntry.WithError(err).Debug("Failed to parse YAML in value, skipping nested key extraction")
		return
	}

	switch v := parsed.(type) {
	case map[string]interface{}:
		collectKeysFromMap(v, keys, logEntry, depth)
	case []interface{}:
		for i, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				collectKeysFromMap(m, keys, logEntry.WithField("array_index", i), depth)
			}
		}
	}
}

func collectKeysFromMap(m map[string]interface{}, keys *[]string, logEntry *logrus.Entry, depth int) {
	for key := range m {
		*keys = append(*keys, key)
		extractNestedKeys(m[key], keys, logEntry.WithField("nested_key", key), depth)
	}
}
