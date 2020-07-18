package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type config struct {
	LogLevel           string `json:"log_level"`
	LogFormat          string `json:"log_format"`
	DatabaseName       string `json:"database_name"`
	DatabaseUser       string `json:"database_user"`
	DatabaseHost       string `json:"database_host"`
	DatabasePassword   string `json:"database_password"`
	GithubClientID     string `json:"github_client_id"`
	GithubClientSecret string `json:"github_client_secret"`
	ServerSecret       string `json:"server_secret,required"`
	StoriesPerPage     int    `json:"stories_per_page"`
	Addr               string `json:"addr"`
}

func main() {
	// setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cfg := config{
		LogLevel:         "info",
		LogFormat:        "json",
		DatabaseName:     "tabloid",
		DatabaseUser:     "postgres",
		DatabasePassword: "postgres",
		DatabaseHost:     "127.0.0.1",
		StoriesPerPage:   10,
		Addr:             "localhost:8080",
	}

	err := cfg.load()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot read configuration")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Str("input", cfg.LogLevel).Msg("Cannot parse log level")
	}
	zerolog.SetGlobalLevel(level)

	var logger zerolog.Logger
	if cfg.LogFormat == "" || cfg.LogFormat == "json" {
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	// setup database
	pgcfg := fmt.Sprintf(
		"user=%v dbname=%v sslmode=disable password=%v host=%v",
		cfg.DatabaseUser,
		cfg.DatabaseName,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
	)
	pg := pgstore.New(pgcfg)

	// setup authentication
	ll := logger.With().Str("component", "github auth").Logger()
	authService := github_auth.New(cfg.ServerSecret, cfg.GithubClientID, cfg.GithubClientSecret, ll)

	// fire the server
	s := tabloid.NewServer(&tabloid.ServerConfig{Addr: cfg.Addr, StoriesPerPage: cfg.StoriesPerPage}, logger, pg, authService)
	err = s.Prepare()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot prepare server")
	}

	err = s.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot start server")
	}
}

func (c *config) load() error {
	f, err := os.Open("config.json")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		return err
	}

	v := os.Getenv("LOG_LEVEL")
	if v != "" {
		c.LogLevel = v
	}

	v = os.Getenv("LOG_FORMAT")
	if v != "" {
		c.LogFormat = v
	}

	v = os.Getenv("DATABASE_NAME")
	if v != "" {
		c.DatabaseName = v
	}

	v = os.Getenv("DATABASE_USER")
	if v != "" {
		c.DatabaseUser = v
	}

	v = os.Getenv("DATABASE_HOST")
	if v != "" {
		c.DatabaseHost = v
	}

	v = os.Getenv("DATABASE_PASSWORD")
	if v != "" {
		c.DatabasePassword = v
	}

	v = os.Getenv("GITHUB_CLIENT_ID")
	if v != "" {
		c.GithubClientID = v
	}

	v = os.Getenv("GITHUB_CLIENT_SECRET")
	if v != "" {
		c.GithubClientSecret = v
	}

	v = os.Getenv("SERVER_SECRET")
	if v != "" {
		c.ServerSecret = v
	}

	v = os.Getenv("STORIES_PER_PAGE")
	if v != "" {
		vi, err := strconv.Atoi(v)
		if err != nil {
			return err
		}

		c.StoriesPerPage = vi
	}

	v = os.Getenv("ADDR")
	if v != "" {
		c.Addr = v
	}

	if c.ServerSecret == "" {
		return fmt.Errorf("missing config 'server secret'")
	}

	if c.GithubClientID == "" {
		return fmt.Errorf("missing config 'github client id'")
	}

	if c.GithubClientSecret == "" {
		return fmt.Errorf("missing config 'github client secret'")
	}

	return nil
}
