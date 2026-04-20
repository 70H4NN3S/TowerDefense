// Package config loads and validates runtime configuration from environment
// variables. It returns an immutable Config struct; callers must not mutate it.
package config

import (
	"errors"
	"fmt"
	"os"
)

// Config holds all runtime configuration. Fields are read-only after Load returns.
type Config struct {
	DatabaseURL string
	JWTSecret   string
	ListenAddr  string
	LogLevel    string
}

// Load reads configuration from environment variables, applies defaults for
// optional fields, and validates all required fields. All validation errors
// are collected and returned together so the operator sees every problem at
// once rather than fixing one at a time.
func Load() (*Config, error) {
	return loadFrom(os.Getenv)
}

// loadFrom is the testable core of Load; it accepts any string-lookup function.
func loadFrom(getenv func(string) string) (*Config, error) {
	c := &Config{
		DatabaseURL: getenv("DATABASE_URL"),
		JWTSecret:   getenv("JWT_SECRET"),
		ListenAddr:  orDefault(getenv("LISTEN_ADDR"), ":8080"),
		LogLevel:    orDefault(getenv("LOG_LEVEL"), "info"),
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.JWTSecret == "" {
		errs = append(errs, errors.New("JWT_SECRET is required"))
	} else if len(c.JWTSecret) < 32 {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(c.JWTSecret)))
	}
	return errors.Join(errs...)
}

func orDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}
