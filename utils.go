package main

import (
	"fmt"
	"strings"
	"time"
	"unsafe"
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

func estimateCacheSize(cacheData map[string]*SecretKeys) uint64 {
	var size uint64
	size += uint64(unsafe.Sizeof(cacheData))
	for path, secretKeys := range cacheData {
		size += uint64(len(path))
		size += uint64(unsafe.Sizeof(secretKeys))
		if secretKeys != nil {
			size += estimateSliceSize(secretKeys.AllKeys)
			size += uint64(len(secretKeys.SearchString))
		}
	}
	return size
}

func estimateSliceSize(slice []string) uint64 {
	var size uint64
	size += uint64(unsafe.Sizeof(slice))
	for _, s := range slice {
		size += uint64(len(s))
	}
	return size
}
