# Tabloid

[![CircleCI](https://circleci.com/gh/jhchabran/tabloid.svg?style=svg&circle-token=533494a13f23294eba935427d6666c2b6eea5ae3)](https://circleci.com/gh/jhchabran/tabloid)
[![Documentation](https://godoc.org/github.com/tabloid/jhchabran?status.svg)](http://godoc.org/github.com/jhchabran/tabloid)

## Objective

Tabloid is a simple, minimalistic Hackernews engine written in Go. It's primary intent is to be used for small
communities, such as the internal newsboard for a company.

As such, standard library is preferred over any other fancy library and deps are kept to the strict minimum. Same rules
applies for the HTML, CSS and Javascript.

## Running the code

Presently, Tabloid is in its early stages and as such, is a very minimal feature set, making it unsuitable for real
production use.

```
make migrate
go run cmd/main.go
open "http://localhost:8080"
```

## Todo

### Features

- [x] should create user when login in.
  - [ ] extraxt email as well
- [x] should update logged at when signing in if already exists

### Bugs

- [ ] fails to load user data when asked to login on Github. Will work fine if already logged in.
