package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/github_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type config struct {
	ClientSecret string `json:"clientSecret"`
	ClientID     string `json:"clientID"`
	ServerSecret string `json:"serverSecret"`
}

func main() {
	// setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	debug := flag.Bool("debug", false, "sets log level to debug")
	jsonlogs := flag.Bool("jsonlogs", false, "json log format (for production)")

	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var logger zerolog.Logger
	if *jsonlogs {
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		output.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		}
		output.FormatMessage = func(i interface{}) string {
			return fmt.Sprintf("***%s****", i)
		}
		output.FormatFieldName = func(i interface{}) string {
			return fmt.Sprintf("%s:", i)
		}
		output.FormatFieldValue = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("%s", i))
		}

		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if e := log.Debug(); e.Enabled() {
		// Compute log output only if enabled.
		value := "bar"
		e.Str("foo", value).Msg("some debug message")
	}

	// setup database
	pg := pgstore.New("user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	config, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err)
	}

	// setup authentication
	authService := github_auth.New(config.ServerSecret, config.ClientID, config.ClientSecret)

	// fire the server
	s := tabloid.NewServer(":8080", logger, pg, authService)
	err = s.Prepare()
	if err != nil {
		log.Fatal().Err(err)
	}

	err = s.Start()
	if err != nil {
		log.Fatal().Err(err)
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
