// +build integration

package tabloid

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
	assert.Nil(t, s.Prepare())

	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}
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
	assert.Nil(t, s.Prepare())

	ts := httptest.NewServer(s)
	defer ts.Close()

	err = s.InsertStory(&Story{
		Title: "Foobar",
		URL:   "http://foobar.com",
	})

	assert.Nil(t, err)

	resp, err := http.Get(ts.URL)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	assert.True(t, strings.Contains(string(body), "foobar"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com"))
}
