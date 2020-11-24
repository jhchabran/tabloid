# Tabloid

[![CircleCI](https://circleci.com/gh/jhchabran/tabloid.svg?style=svg&circle-token=533494a13f23294eba935427d6666c2b6eea5ae3)](https://circleci.com/gh/jhchabran/tabloid)
[![Documentation](https://godoc.org/github.com/jhchabran/tabloid?status.svg)](http://godoc.org/github.com/jhchabran/tabloid)

_Presently, Tabloid is in its early stages, making it a bit rough for production use. APIs and DB schemas may break._

## Purpose

Tabloid is a simple, minimalistic Hackernews engine written in Go. It is designed toward small private
communities, such as a company in its early stages or a group of friends. It makes it easy to set up a private newsboard and to adapt it to your context.

In most scenarios, the base idea stays the same: a group of people exchange and score links, and post comments about them, in a tree form. But often, what drives adoption and/or what makes it valuable for a particular community is tailoring it to that community pre-existing tools and ecosystem.

## How it works

To allow for extensibility, Tabloid goes for a "library" model, where you simply create a `main.go` , import tabloid and write your custom behaviour:

```go
package main

import (
    // ...
    "github.com/jhchabran/tabloid"
)


func main() {
    // load the config
    cfg := cmd.DefaultConfig()
    err := cfg.Load()
    if err != nil {
        log.Fatal().Err(err).Msg("Cannot read configuration")
    }
    logger := cmd.SetupLogger(cfg)

    // setup database
    pg := pgstore.New(cfg.DatabaseURL)

    // setup authentication
    authService := github_auth.New(cfg.ServerSecret, cfg.GithubClientID, cfg.GithubClientSecret, ll)

    // create the server
    s := tabloid.NewServer(&tabloid.ServerConfig{Addr: cfg.Addr, StoriesPerPage: cfg.StoriesPerPage}, logger, pg, authService)

    // 🔥 do something every time a story is submitted
    s.AddStoryHook(func(story *tabloid.Story) error {
        s.Logger.Debug().Msg("Adding a story hook")
        // example: post the story on a #share channel on slack
        return nil
    })

    // 🔥 do something every time a comment is submitted
    s.AddCommentHook(func(story *tabloid.Story, comment *tabloid.Comment) error {
        s.Logger.Debug().Msg("Adding a comment hook")
        // example: post the comment on a #share channel on slack and ping users mentioned in the comments
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

```

From there, this file can be versioned and Tabloid updates are just a matter of updating your go modules and your customizations are self-contained.

## Deploying it

Presently, it's not streamlined at all, as it's still the early stages and no stable releases had been made. The main goal there is to provide an example repository that can be forked, modified and deployed to common cloud providers with a single button (See [#51](https://github.com/jhchabran/tabloid/issues/51), [#8](https://github.com/jhchabran/tabloid/issues/8))

Still, for now it can be easily deployed on Heroku with the provided Dockerfile.

## Reasoning

Communities using Tabloid may come from different software backgrounds, which is why Tabloid isn't using any kind of framework. Everybody should be able to contribute and frameworks are usually getting in the way when it comes to add that little feature that would makes sense in your context.

Also, not relying on any framework makes the code a bit more resiliant to time. Nobody likes to hack around outdated libs and the feature-set is small enough to deal with it without it. It doesn't have usual abstractions for such an app and yes it could be written in less than n lines with library X but the idea is that the code is almost self-contained.

Not knowing too much about Go shouldn't be an entry barrier. Similarly, the front-end code aims to be as simple as possible. Pure HTML should be enough for this kind of web application.

## Contributing

See the [issues](https://github.com/jhchabran/tabloid/issues) to report a bug, open a feature request or simply if you want to find something to contribute on. [Good first issues](https://github.com/jhchabran/tabloid/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) are a good way to start.

### Running the code

```
make migrate
go run cmd/server/main.go
open "http://localhost:8080"
```

### Config

See `config.example.json`.

- `LOG_LEVEL` sets log level; defaults to `info`
- `LOG_FORMAT` sets log format; defaults to `json`
- `PORT` sets the port to listen for incoming requests, supersedes `ADDR`.
- `ADDR` sets the address to listen for incoming requests
- `DATABASE_URL` sets the database url string, supersedes other database settings below.
- `DATABASE_NAME`sets the database name
- `DATABASE_USER`sets the database user
- `DATABASE_HOST`sets the database host
- `DATABASE_PASSWORD`sets the database password
- `GITHUB_CLIENT_ID`sets the Github client ID
- `GITHUB_CLIENT_SECRET`sets the Github client secret
- `SERVER_SECRET`sets the server secret for cookies
- `STORIES_PER_PAGE`sets the server number of stories per page; default to `20`.

Configuration for the provided example main (`cmd/server/main.go`), used for dev purpose until we reach a stable release:

- `GH_USERNAMES_JSON` sets the url to a json file representing a table of slack handles github handles, used in my custom hook, for pinging users in comments notifications.
