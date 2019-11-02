// +build integration

package tabloid

import (
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleIndex200(t *testing.T) {
	// TODO there must be a better way to do this
	err := os.Chdir("..")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir("tabloid")
	}()

	s := NewServer("localhost:8081", "user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")

	go func() {
		s.Start()
	}()

	resp, err := http.Get("http://google.fr")
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	s.Stop()
}

func TestHandleIndexStory(t *testing.T) {
	// TODO there must be a better way to do this
	err := os.Chdir("..")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir("tabloid")
	}()

	s := NewServer("localhost:8081", "user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")

	go func() {
		s.Start()
	}()

	// TODO should know when the server is ready instead
	time.Sleep(time.Second)
	err = s.InsertStory(&Story{
		Title: "Foobar",
		URL:   "http://foobar.com",
	})

	assert.Nil(t, err)

	resp, err := http.Get("http://localhost:8081")
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)

	assert.True(t, strings.Contains(string(body), "foobar"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com"))

	s.Stop()
}
