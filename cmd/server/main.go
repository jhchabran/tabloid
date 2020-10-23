package main

import (
	"fmt"
	"os"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/cmd"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/nlopes/slack"
	"github.com/rs/zerolog/log"
)

// For the moment, this code is the one that powers my own instance, for development purpose.
// TODO turn this into a template that people can take inspiration from or even generate their main with.
func main() {
	cfg := cmd.DefaultConfig()
	err := cfg.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot read configuration")
	}
	logger := cmd.SetupLogger(cfg)

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

	// create the server
	s := tabloid.NewServer(&tabloid.ServerConfig{Addr: cfg.Addr, StoriesPerPage: cfg.StoriesPerPage}, logger, pg, authService)

	slackToken := os.Getenv("SLACK_TOKEN")
	api := slack.New(slackToken)
	var cid string
	channels, _, err := api.GetConversations(&slack.GetConversationsParameters{
		Limit: 100,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot connect to slack API")
	}

	for _, channel := range channels {
		if channel.Name == "share" {
			cid = channel.ID
		}
	}
	s.AddStoryHook(func(story *tabloid.Story) error {
		s.Logger.Debug().Msg("Adding a story hook")
		_, _, err := api.PostMessage(cid, slack.MsgOptionText(
			story.URL+
				" submitted by "+
				story.Author+
				"\nComments "+
				cfg.RootURL+
				"/stories/"+
				story.ID+
				"/comments",
			false))
		if err != nil {
			return err
		}
		return nil
	})

	s.AddCommentHook(func(story *tabloid.Story, comment *tabloid.Comment) error {
		s.Logger.Debug().Msg("Adding a comment hook")
		_, _, err := api.PostMessage(cid, slack.MsgOptionText(
			"Comment submitted by "+
				comment.Author+
				" on \""+
				story.Title+
				"\": "+
				cfg.RootURL+
				"/stories/"+
				story.ID+
				"/comments",
			false))
		if err != nil {
			return err
		}
		return nil
	})

	err = s.Prepare()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot prepare server")
	}

	err = s.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot start server")
	}
}
