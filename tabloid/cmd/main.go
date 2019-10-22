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

	i := tabloid.NewItem("somee title",
		"some body",
		"jh")

	err = s.InsertItem(i)
	if err != nil {
		log.Fatal(err)
	}
}
