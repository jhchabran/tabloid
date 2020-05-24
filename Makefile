migrate:
	migrate -path ./db/migrations -database "postgres://postgres:postgres@127.0.0.1:5432/tabloid?sslmode=disable" up

migrate_test:
	migrate -path ./db/migrations -database "postgres://postgres:postgres@127.0.0.1:5432/tabloid_test?sslmode=disable" up

integration_test:
	go test -tags=integration ./...

test:
	go test ./... -v
