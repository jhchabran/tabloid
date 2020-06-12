package integration

import (
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/fake_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	suite.Suite
	pgStore    *pgstore.PGStore
	fakeAuth   *fake_auth.Handler
	testServer *httptest.Server
	server     *tabloid.Server
	// used to reference an existing user
	existingUserID int64
}

func (suite *IntegrationTestSuite) SetupTest() {
	err := os.Chdir("..")
	if err != nil {
		suite.FailNow("%v", err)
	}
	suite.pgStore = pgstore.New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	sessionStore := sessions.NewCookieStore([]byte("test"))

	suite.fakeAuth = fake_auth.New(sessionStore)
	suite.server = tabloid.NewServer("localhost:8081", suite.pgStore, suite.fakeAuth)
	suite.testServer = httptest.NewServer(suite.server)
	suite.fakeAuth.SetServerURL(suite.testServer.URL)

	assert.Nil(suite.T(), suite.server.Prepare())
	// inserts first user used for existing posts, with id=1
	err = suite.pgStore.DB().Get(&suite.existingUserID,
		"INSERT INTO users (name, email, created_at, last_login_at) VALUES ('alpha', 'alpha@email.com', $1, $2) RETURNING id",
		time.Now(), time.Now())
	suite.NoError(err, suite.T())
}

func (suite *IntegrationTestSuite) TearDownTest() {
	defer suite.testServer.Close()
	os.Chdir("integration")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE stories;")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE comments;")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE users;")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (suite *IntegrationTestSuite) TestEmptyIndex() {
	t := suite.T()

	resp, err := http.Get(suite.testServer.URL)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}
}

func (suite *IntegrationTestSuite) TestIndexWithStory() {
	t := suite.T()

	err := suite.pgStore.InsertStory(&tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Score:     1,
		Body:      "Foobaring",
		AuthorID:  suite.existingUserID,
		CreatedAt: time.Now(),
	})
	assert.NoError(t, err)

	resp, err := http.Get(suite.testServer.URL)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	assert.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	assert.True(t, strings.Contains(string(body), "foobar"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com"))
}

func (suite *IntegrationTestSuite) TestSubmitStory() {
	t := suite.T()

	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// test for the form being rendered
	resp, err = client.Get(suite.testServer.URL + "/submit")
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	assert.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	assert.True(t, strings.Contains(string(body), "id=\"submit-form\""))

	// test for submitting the form
	values := url.Values{
		"title": []string{"Captain Nemo"},
		"url":   []string{"http://duckduckgo.com"},
		"body":  []string{"foobar"},
	}
	resp, err = client.PostForm(suite.testServer.URL+"/submit", values)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	assert.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	assert.True(t, strings.Contains(string(body), "Captain Nemo"))
	assert.True(t, strings.Contains(string(body), "href=\"http://duckduckgo.com\""))
}
