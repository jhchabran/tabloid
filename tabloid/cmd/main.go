package main

import (
	"log"

	"github.com/jhchabran/tabloid/tabloid"
)

func main() {
	s := tabloid.NewServer(":8080")
	err := s.Start()
	if err != nil {
		log.Fatal(err)
	}

	i := tabloid.NewStory("somee title",
		"some body",
		"jh")

	err = s.InsertStory(i)
	if err != nil {
		log.Fatal(err)
	}
}
