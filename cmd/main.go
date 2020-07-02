package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// default to info
	rawLevel := os.Getenv("LOG_LEVEL")
	if rawLevel == "" {
		rawLevel = "info"
	}

	level, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.Fatal().Err(err).Str("input", os.Getenv("LOG_LEVEL")).Msg("Cannot parse LOG_LEVEL")
	}
	zerolog.SetGlobalLevel(level)

	var logger zerolog.Logger
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat == "" || logFormat == "json" {
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	// setup database
	pg := pgstore.New("user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")

	// load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err)
	}

	// setup authentication
	ll := logger.With().Str("component", "github auth").Logger()
	authService := github_auth.New(config.ServerSecret, config.ClientID, config.ClientSecret, ll)

	// fire the server
	s := tabloid.NewServer(config, logger, pg, authService)
	err = s.Prepare()
	if err != nil {
		log.Fatal().Err(err)
	}

	err = s.Start()
	if err != nil {
		log.Fatal().Err(err)
	}
}

func loadConfig() (*tabloid.ServerConfig, error) {
	config := &tabloid.ServerConfig{}
	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(b, config); err != nil {
		return nil, err
	}

	return config, nil
}
