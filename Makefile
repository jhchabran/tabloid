migrate:
	migrate -path ./db/migrations -database "postgres://postgres:postgres@127.0.0.1:5432/tabloid?sslmode=disable" up
