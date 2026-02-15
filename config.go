package main

import (
	"io"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
)

type Config struct {
	VaultAddress       string
	VaultToken         string
	VaultMountPoint    string
	LocalServerAddress string
	MaxGoroutines      int
	LogLevel           string
	LogFilePath        string
}

var (
	logger      *logrus.Logger
	cfg         *Config
	vaultClient *api.Client
	cache       *Cache
	rebuildWg   sync.WaitGroup
	logFile     *os.File
)

func init() {
	cfg = loadConfig()
	logger = setupLogger()
	vaultClient = setupVaultClient()
	cache = &Cache{data: make(map[string]*SecretKeys)}
}

func loadConfig() *Config {
	logLevel := getEnv("LOG_LEVEL", "info")
	logFilePath := getEnv("LOG_FILE_PATH", "/tmp/vault_search.log")
	maxGoroutinesStr := getEnv("MAX_GOROUTINES", "15")
	maxGoroutines, err := strconv.Atoi(maxGoroutinesStr)
	if err != nil || maxGoroutines <= 0 {
		maxGoroutines = 10
	}

	return &Config{
		VaultAddress:       getEnv("VAULT_ADDR", "https://vault.offline.shelopes.com"),
		VaultToken:         os.Getenv("VAULT_TOKEN"),
		VaultMountPoint:    getEnv("VAULT_MOUNT_POINT", "kv"),
		LocalServerAddress: getEnv("LOCAL_SERVER_ADDR", "localhost:8080"),
		MaxGoroutines:      maxGoroutines,
		LogLevel:           logLevel,
		LogFilePath:        logFilePath,
	}
}

func setupLogger() *logrus.Logger {
	log := logrus.New()
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	log.SetFormatter(&logrus.JSONFormatter{})

	if cfg.LogFilePath != "" {
		if err := os.MkdirAll(path.Dir(cfg.LogFilePath), 0750); err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
		f, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		logFile = f
		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
	}

	return log
}

func setupVaultClient() *api.Client {
	config := api.DefaultConfig()
	config.Address = cfg.VaultAddress

	client, err := api.NewClient(config)
	if err != nil {
		logger.Fatalf("Failed to create Vault client: %v", err)
	}

	client.SetToken(cfg.VaultToken)
	return client
}

func closeLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}
