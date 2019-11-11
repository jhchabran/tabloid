# Tabloid

[![Build Status](https://travis-ci.org/jhchabran/tabloid.svg?branch=master)](https://travis-ci.org/jhchabran/tabloid)
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
