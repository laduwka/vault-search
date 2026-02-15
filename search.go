package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
)

type SearchParams struct {
	Term   string
	Regexp string
	InPath string
	Sort   string
	ShowUI bool
}

type SearchResult struct {
	Matches     []string
	VaultUIBase string
}

func performSearch(params *SearchParams, regex *regexp.Regexp, ctx context.Context) (*SearchResult, error) {
	vaultUIBaseURL := fmt.Sprintf("%s/ui/vault/secrets/%s/show", cfg.VaultAddress, cfg.VaultMountPoint)

	var contentMatches []string
	var pathMatches []string

	cache.RLock()
	defer cache.RUnlock()

	estimatedCap := len(cache.data) / 10
	if estimatedCap < 8 {
		estimatedCap = 8
	}

	eg, egCtx := errgroup.WithContext(ctx)

	if params.Term != "" || params.Regexp != "" {
		eg.Go(func() error {
			local := make([]string, 0, estimatedCap)
			for secretPath, secretKeys := range cache.data {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				default:
				}

				if matchSecret(secretPath, secretKeys, params, regex) {
					local = append(local, secretPath)
				}
			}
			contentMatches = local
			return nil
		})
	}

	if params.InPath != "" {
		eg.Go(func() error {
			local := make([]string, 0, estimatedCap)
			for secretPath := range cache.data {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				default:
				}

				if matchInPath(secretPath, params.InPath) {
					local = append(local, secretPath)
				}
			}
			pathMatches = local
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	matches := determineMatches(params, contentMatches, pathMatches)

	sort.Strings(matches)
	if params.Sort == "desc" {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
	}

	if params.ShowUI {
		for i, secretPath := range matches {
			matches[i] = fmt.Sprintf("%s/%s", vaultUIBaseURL, secretPath)
		}
	}

	return &SearchResult{
		Matches:     matches,
		VaultUIBase: vaultUIBaseURL,
	}, nil
}

func matchSecret(path string, keys *SecretKeys, params *SearchParams, regex *regexp.Regexp) bool {
	if params.Term != "" {
		return strings.Contains(keys.SearchString, strings.ToLower(params.Term))
	}

	if params.Regexp != "" && regex != nil {
		return regex.MatchString(keys.SearchString)
	}

	return false
}

func matchInPath(secretPath, inPath string) bool {
	if secretPath == inPath {
		return true
	}
	if strings.HasPrefix(secretPath, inPath+"/") {
		return true
	}
	if strings.HasSuffix(secretPath, "/"+inPath) {
		return true
	}
	return strings.Contains(secretPath, "/"+inPath+"/")
}

func determineMatches(params *SearchParams, contentMatches, pathMatches []string) []string {
	var matches []string

	hasContentSearch := params.Term != "" || params.Regexp != ""
	hasPathSearch := params.InPath != ""

	if hasContentSearch && hasPathSearch {
		contentSet := make(map[string]struct{})
		for _, path := range contentMatches {
			contentSet[path] = struct{}{}
		}
		for _, path := range pathMatches {
			if _, exists := contentSet[path]; exists {
				matches = append(matches, path)
			}
		}
	} else if hasContentSearch {
		matches = contentMatches
	} else if hasPathSearch {
		matches = pathMatches
	}

	return matches
}
