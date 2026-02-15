package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type SecretKeys struct {
	AllKeys      []string
	SearchString string
}

type Cache struct {
	sync.RWMutex
	data           map[string]*SecretKeys
	buildStartTime time.Time
	buildEndTime   time.Time
	isRebuilding   int32
	totalSecrets   int64
	fetchedSecrets int64
	totalKeys      int64
}

func rebuildCache() error {
	c := cache
	if !atomic.CompareAndSwapInt32(&c.isRebuilding, 0, 1) {
		logger.Info("Cache rebuild is already in progress")
		return nil
	}
	defer atomic.StoreInt32(&c.isRebuilding, 0)

	c.Lock()
	c.buildStartTime = time.Now()
	c.Unlock()

	logger.Info("Starting cache rebuild")

	tempCache := make(map[string]*SecretKeys)
	pathsCh := make(chan string, 1000)
	errCh := make(chan error, 1)
	listingResultCh := make(chan error, 1)

	var wg sync.WaitGroup

	listCtx, listCancel := context.WithCancel(context.Background())
	defer listCancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		listAllSecrets(listCtx, "", pathsCh, errCh)
		close(pathsCh)
	}()

	go func() {
		wg.Wait()
		select {
		case err := <-errCh:
			listingResultCh <- err
		default:
			listingResultCh <- nil
		}
	}()

	sem := make(chan struct{}, cfg.MaxGoroutines)
	var mu sync.Mutex

	var totalSecrets int64
	totalKeys := int64(0)
	atomic.StoreInt64(&c.fetchedSecrets, 0)

	eg, ctx := errgroup.WithContext(context.Background())

	for secretPath := range pathsCh {
		atomic.AddInt64(&totalSecrets, 1)
		secretPath := secretPath
		sem <- struct{}{}
		eg.Go(func() error {
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				logEntry := logger.WithField("secret_path", secretPath)
				logEntry.Debug("Fetching secret")

				secret, err := vaultClient.Logical().Read(fmt.Sprintf("%s/data/%s", cfg.VaultMountPoint, secretPath))
				if err != nil {
					if isPermissionDenied(err) {
						logEntry.WithError(err).Warn("Access denied for secret")
						return nil
					}
					logEntry.WithError(err).Error("Failed to read secret")
					return nil
				}

				if secret == nil || secret.Data == nil {
					logEntry.Warn("Secret data is nil")
					return nil
				}

				data, ok := secret.Data["data"].(map[string]interface{})
				if !ok {
					logEntry.Error("Invalid data format in secret")
					return nil
				}

				allKeys := extractKeysFromValue(data, logEntry)
				searchString := buildSearchString(secretPath, allKeys)

				mu.Lock()
				tempCache[secretPath] = &SecretKeys{
					AllKeys:      allKeys,
					SearchString: searchString,
				}
				totalKeys += int64(len(allKeys))
				mu.Unlock()

				fetched := atomic.AddInt64(&c.fetchedSecrets, 1)
				total := atomic.LoadInt64(&totalSecrets)
				if fetched%100 == 0 || fetched == total {
					logger.WithFields(logrus.Fields{
						"fetched_secrets": fetched,
						"total_secrets":   total,
					}).Info("Fetched secrets progress")
				}

				return nil
			}
		})
	}

	if err := eg.Wait(); err != nil {
		logger.WithError(err).Error("Error during cache rebuild")
	}

	listingErr := <-listingResultCh
	if listingErr != nil {
		logger.WithError(listingErr).Error("Error during listing secrets")
		return listingErr
	}

	atomic.StoreInt64(&c.totalSecrets, atomic.LoadInt64(&totalSecrets))

	c.Lock()
	c.data = tempCache
	c.buildEndTime = time.Now()
	c.Unlock()
	atomic.StoreInt64(&c.totalKeys, totalKeys)

	logger.WithField("total_keys", totalKeys).Info("Cache rebuild completed")
	return nil
}

func listAllSecrets(ctx context.Context, currentPath string, pathsCh chan<- string, errCh chan<- error) {
	logEntry := logger.WithField("current_path", currentPath)
	logEntry.Debug("Listing secrets")

	select {
	case <-ctx.Done():
		return
	default:
	}

	secretList, err := vaultClient.Logical().List(fmt.Sprintf("%s/metadata/%s", cfg.VaultMountPoint, currentPath))
	if err != nil {
		logEntry.WithError(err).Error("Failed to list secrets at path")
		select {
		case errCh <- fmt.Errorf("failed to list secrets at path %s: %w", currentPath, err):
		default:
		}
		return
	}
	if secretList == nil || secretList.Data == nil {
		logEntry.Debug("No secrets found at path")
		return
	}

	keys, ok := secretList.Data["keys"].([]interface{})
	if !ok {
		logEntry.Warn("No keys found in secret data")
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.MaxGoroutines)

	for _, key := range keys {
		select {
		case <-ctx.Done():
			break
		default:
		}

		keyStr, ok := key.(string)
		if !ok {
			logger.WithField("key", key).Warn("Key is not a string")
			continue
		}
		fullPath := path.Join(currentPath, keyStr)
		if strings.HasSuffix(keyStr, "/") {
			fullPath = currentPath + "/" + keyStr
			fullPath = strings.TrimSuffix(fullPath, "/")
			sem <- struct{}{}
			wg.Add(1)
			go func(p string) {
				defer func() { <-sem }()
				defer wg.Done()
				listAllSecrets(ctx, p, pathsCh, errCh)
			}(fullPath)
		} else {
			logger.WithField("secret_path", fullPath).Debug("Found secret")
			pathsCh <- fullPath
		}
	}

	wg.Wait()
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	var respErr *api.ResponseError
	if errors.As(err, &respErr) && respErr.StatusCode == 403 {
		return true
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "403")
}

func buildSearchString(path string, keys []string) string {
	var sb strings.Builder
	sb.WriteString(strings.ToLower(path))
	sb.WriteString(" ")
	for _, key := range keys {
		sb.WriteString(strings.ToLower(key))
		sb.WriteString(" ")
	}
	return sb.String()
}
