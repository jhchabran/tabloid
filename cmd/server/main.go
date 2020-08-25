package main

import (
	"fmt"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/cmd"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := cmd.DefaultConfig()
	err := cfg.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot read configuration")
	}
	logger := cmd.SetupLogger(cfg)
	logger.Info().Msg("Seeding database")

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
