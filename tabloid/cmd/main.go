package main

import (
	"log"

	"github.com/jhchabran/tabloid/tabloid"
)

func main() {
	s := tabloid.NewServer(":8080", "user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	err := s.Start()
	if err != nil {
		log.Fatal(err)
	}
}
