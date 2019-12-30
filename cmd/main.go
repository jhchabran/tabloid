package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/pgstore"
)

type config struct {
	ClientSecret string `json:"clientSecret"`
	ClientID     string `json:"clientID"`
	ServerSecret string `json:"serverSecret"`
}

func main() {
	pg := pgstore.New("user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	authService := github_auth.New(config.ServerSecret, config.ClientID, config.ClientSecret)
	s := tabloid.NewServer(":8080", pg, authService)
	err = s.Prepare()
	if err != nil {
		log.Fatal(err)
	}

	err = s.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func loadConfig() (*config, error) {
	config := &config{}
	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(b, config); err != nil {
		return nil, err
	}

	return config, nil
}
