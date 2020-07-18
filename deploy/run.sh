#!/bin/sh

set -e

./migrate -path ./db/migrations -database "postgres://$DATABASE_USER:$DATABASE_PASSWORD@$DATABASE_HOST/$DATABASE_NAME?sslmode=disable" up

exec ./tabloid
