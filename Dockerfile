FROM golang:alpine AS build
RUN apk --no-cache add gcc g++ make git curl
WORKDIR /go/src/app
COPY . .
RUN go get ./...
RUN GOOS=linux go build -ldflags="-s -w" -o ./bin/tabloid ./cmd/main.go
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.11.0/migrate.linux-amd64.tar.gz | tar xvz

FROM alpine:3.12
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /go/src/app/bin /app
COPY --from=build /go/src/app/deploy/run.sh /app/run.sh
COPY --from=build /go/src/app/assets /app/assets
COPY --from=build /go/src/app/db /app/db
COPY --from=build /go/src/app/migrate.linux-amd64 /app/migrate

EXPOSE 80
CMD ["/bin/sh", "-c", "/app/run.sh"]
