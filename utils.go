package main

import (
	"fmt"
	"reflect"
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

func estimateValueSize(value interface{}) uint64 {
	var size uint64
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		size += uint64(len(v.String()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		size += uint64(unsafe.Sizeof(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		size += uint64(unsafe.Sizeof(v.Uint()))
	case reflect.Float32, reflect.Float64:
		size += uint64(unsafe.Sizeof(v.Float()))
	case reflect.Bool:
		size += uint64(unsafe.Sizeof(v.Bool()))
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			size += estimateValueSize(v.Index(i).Interface())
		}
	case reflect.Map:
		if m, ok := v.Interface().(map[string]interface{}); ok {
			size += estimateMapSize(m)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			size += estimateValueSize(v.Field(i).Interface())
		}
	default:
		size += uint64(unsafe.Sizeof(value))
	}
	return size
}

func estimateMapSize(m map[string]interface{}) uint64 {
	var size uint64
	size += uint64(unsafe.Sizeof(m))
	for k, v := range m {
		size += uint64(len(k))
		size += estimateValueSize(v)
	}
	return size
}
