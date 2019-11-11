package main

import (
	"log"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/pgstore"
)

func main() {
	pg := pgstore.New("user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	s := tabloid.NewServer(":8080", pg)
	err := s.Prepare()
	if err != nil {
		log.Fatal(err)
	}

	err = s.Start()
	if err != nil {
		log.Fatal(err)
	}
}
