migrate:
	migrate -path ./db/migrations -database "postgres://postgres:postgres@127.0.0.1:5432/tabloid?sslmode=disable" up

migrate_test:
	migrate -path ./db/migrations -database "postgres://postgres:postgres@127.0.0.1:5432/tabloid_test?sslmode=disable" up

integration_test:
	go test -tags=integration ./...

test:
	go test ./...

reset_db:
	dropdb -U postgres -h localhost -p 5432 tabloid
	createdb -U postgres -h localhost -p 5432 tabloid
	dropdb -U postgres -h localhost -p 5432 tabloid_test
	createdb -U postgres -h localhost -p 5432 tabloid_test
	make migrate
	make migrate_test
