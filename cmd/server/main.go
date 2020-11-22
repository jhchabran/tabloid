// This package is a playground for my current experimentations and showcase how Tabloid could be used by communities.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/cmd"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"golang.org/x/net/context/ctxhttp"
)

var usernameNotFoundError = errors.New("username not found")
var slackUserNotFoundError = errors.New("username not found")

// SlackUsernameResolver provides an way to convert usernames from one platform to one another.
type SlackUsernameResolver struct {
	slackNames    map[string]string
	slackUsersIDs map[string]string
}

func newSlackUsernameResolver(ctx context.Context, client *slack.Client, jsonUrl string) (*SlackUsernameResolver, error) {
	res, err := ctxhttp.Get(ctx, http.DefaultClient, jsonUrl)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var r SlackUsernameResolver
	err = json.Unmarshal(data, &r.slackNames)
	if err != nil {
		return nil, err
	}

	users, err := client.GetUsers()
	if err != nil {
		return nil, err
	}

	var missingUsers []string
	r.slackUsersIDs = map[string]string{}
outer:
	for _, n := range r.slackNames {
		for _, u := range users {
			if u.Name == n {
				r.slackUsersIDs[n] = u.ID
				continue outer
			}
		}

		missingUsers = append(missingUsers, n)

	}

	if len(r.slackNames) != len(r.slackUsersIDs) {
		return nil, fmt.Errorf("some usernames are missing: %v", missingUsers)
	}

	return &r, nil
}

func (r *SlackUsernameResolver) Resolve(username string) (string, error) {
	if name, ok := r.slackNames[username]; ok {
		return r.slackUsersIDs[name], nil
	} else {
		return "", usernameNotFoundError
	}
}

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
	var pg *pgstore.PGStore
	if cfg.DatabaseURL != "" {
		pg = pgstore.New(cfg.DatabaseURL)
	} else {
		pgcfg := fmt.Sprintf(
			"user=%v dbname=%v sslmode=disable password=%v host=%v",
			cfg.DatabaseUser,
			cfg.DatabaseName,
			cfg.DatabasePassword,
			cfg.DatabaseHost,
		)
		pg = pgstore.New(pgcfg)
	}

	// setup authentication
	ll := logger.With().Str("component", "github auth").Logger()
	authService := github_auth.New(cfg.ServerSecret, cfg.GithubClientID, cfg.GithubClientSecret, ll)

	// create the server
	s := tabloid.NewServer(&tabloid.ServerConfig{Addr: cfg.Addr, StoriesPerPage: cfg.StoriesPerPage}, logger, pg, authService)

	// create the slack client; needed scope channel list, user list, post messages
	slackToken := os.Getenv("SLACK_TOKEN")
	api := slack.New(slackToken)

	// find the share channel
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

	// load the username resolver
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()
	resolver, err := newSlackUsernameResolver(ctx, api, os.Getenv("GH_USERNAMES_JSON"))
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot populate the slack to github username table")
	}

	userFmt := func(uid string) string {
		return "<@" + uid + ">"
	}

	s.AddStoryHook(func(story *tabloid.Story) error {
		s.Logger.Debug().Msg("Adding a story hook")
		uid, err := resolver.Resolve(story.Author)
		if err != nil {
			return err
		}

		_, _, err = api.PostMessage(cid, slack.MsgOptionText(
			story.URL+
				" submitted by "+
				userFmt(uid)+
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
		uid, err := resolver.Resolve(comment.Author)
		if err != nil {
			return err
		}

		_, _, err = api.PostMessage(cid, slack.MsgOptionText(
			"Comment submitted by "+
				userFmt(uid)+
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

	// Prepare and start the server
	err = s.Prepare()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot prepare server")
	}

	err = s.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("Cannot start server")
	}
}
