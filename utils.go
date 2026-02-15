package main

import (
	"fmt"
	"strings"
	"time"
)

func humanReadableDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	seconds := int64(d.Seconds()) % 60
	minutes := int64(d.Minutes()) % 60
	hours := int64(d.Hours()) % 24
	days := int64(d.Hours()) / 24

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}

func roundDurationToTenSeconds(d time.Duration) time.Duration {
	seconds := int64(d.Seconds())
	roundedSeconds := (seconds / 10) * 10
	return time.Duration(roundedSeconds) * time.Second
}

const (
	mapEntryOverhead = 100 // approximate per-entry overhead for Go map buckets
	stringHeaderSize = 16  // string header (pointer + length)
	sliceHeaderSize  = 24  // slice header (pointer + length + capacity)
	pointerSize      = 8   // pointer to SecretKeys
)

func estimateCacheSize(cacheData map[string]*SecretKeys) uint64 {
	var size uint64
	for path, secretKeys := range cacheData {
		size += mapEntryOverhead
		size += stringHeaderSize + uint64(len(path))
		size += pointerSize
		if secretKeys != nil {
			size += stringHeaderSize + uint64(len(secretKeys.SearchString))
			size += sliceHeaderSize
			for _, s := range secretKeys.AllKeys {
				size += stringHeaderSize + uint64(len(s))
			}
		}
	}
	return size
}
