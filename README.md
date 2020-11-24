# Tabloid

[![CircleCI](https://circleci.com/gh/jhchabran/tabloid.svg?style=svg&circle-token=533494a13f23294eba935427d6666c2b6eea5ae3)](https://circleci.com/gh/jhchabran/tabloid)
[![Documentation](https://godoc.org/github.com/tabloid/jhchabran?status.svg)](http://godoc.org/github.com/jhchabran/tabloid)

## Objective

Tabloid is a simple, minimalistic Hackernews engine written in Go. It's primary intent is to for small
communities, such as the internal newsboard for a company or a community.

Communities using Tabloid may come from different software backgrounds, which explains why Tabloid isn't using any kind of framework. Everybody should be able to contribute and frameworks are usually getting in the way when it comes to add that little feature that would makes sense in your context.

Not relying on any framework makes the code a bit more resiliant to time. Nobody likes to hack around outdated frameworks and the feature-set is small enough to deal with it without it. It's lacking the usual abstactions for such an app and yes it could be written in less than n lines with library X but the idea is that the code is almost self-contained. Not knowing too much about Go shouldn't be an entry barrier.

Similarly, the front-end code aims to be as simple as possible. Pure HTML isn't fancy but it does the job enough for this kind of app. And it's an interesting constraint, at least to me.

## Running the code

Presently, Tabloid is in its early stages and as such, is a very minimal feature set, making it a bit rough for production use. Use it at your own risk.

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

### Bugs

- [ ] fails to load user data when asked to login on Github. Will work fine if already logged in.
