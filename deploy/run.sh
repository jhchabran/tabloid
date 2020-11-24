#!/bin/sh

set -e

if [ -z "$DATABASE_URL" ]
then
    ./migrate -path ./db/migrations -database "postgres://$DATABASE_USER:$DATABASE_PASSWORD@$DATABASE_HOST/$DATABASE_NAME?sslmode=disable" up
else
    ./migrate -path ./db/migrations -database $DATABASE_URL up
fi

exec ./tabloid
