package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type EntryConfig struct {
	NostrRelays   []string `env:"NOSTR_RELAYS" envSeparator:";"`
	PublicAddress string   `env:"PUBLIC_ADDRESS"`
}

type ExitConfig struct {
	NostrRelays     []string `env:"NOSTR_RELAYS" envSeparator:";"`
	NostrPrivateKey string   `env:"NOSTR_PRIVATE_KEY"`
	BackendHost     string   `env:"BACKEND_HOST"`
	BackendScheme   string   `env:"BACKEND_SCHEME"`
	HttpsPort       int32
	HttpsTarget     string
}

// load the and marshal Configuration from .env file from the UserHomeDir
// if this file was not found, fallback to the os environment variables
func LoadConfig[T any]() (*T, error) {
	// load current users home directory as a string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("error loading home directory", err)
	}
	// check if .env file exist in the home directory
	// if it does, load the configuration from it
	// else fallback to the os environment variables
	if _, err := os.Stat(homeDir + "/.env"); err == nil {
		// load configuration from .env file
		return loadFromEnv[T](homeDir + "/.env")
	} else if _, err := os.Stat(".env"); err == nil {
		// load configuration from .env file in current directory
		return loadFromEnv[T]("")
	} else {
		// load configuration from os environment variables
		return loadFromEnv[T]("")
	}
}

// loadFromEnv loads the configuration from the specified .env file path.
// If the path is empty, it does not load any configuration.
// It returns an error if there was a problem loading the configuration.
func loadFromEnv[T any](path string) (*T, error) {
	// check path

	// load configuration from .env file
	err := godotenv.Load()
	if err != nil {
		cfg, err := env.ParseAs[T]()
		if err != nil {
			fmt.Printf("%+v\n", err)
		}
		return &cfg, nil
	}

	// or you can use generics
	cfg, err := env.ParseAs[T]()
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
	return &cfg, nil
}
