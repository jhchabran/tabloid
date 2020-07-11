package integration

import (
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/fake_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog"
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

type testingLogWriter struct {
	suite *suite.Suite
}

func (l *testingLogWriter) Write(p []byte) (n int, err error) {
	str := string(p[0 : len(p)-1]) // drop the final \n
	l.suite.T().Log(str)
	return len(p), nil
}

func (suite *IntegrationTestSuite) SetupTest() {
	// cd .. for assets
	err := os.Chdir("..")
	if err != nil {
		suite.FailNow("%v", err)
	}

	// prepare test config
	config := &tabloid.ServerConfig{
		Addr:           "localhost:8081",
		StoriesPerPage: 3,
	}

	// prepare components
	suite.pgStore = pgstore.New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	sessionStore := sessions.NewCookieStore([]byte("test"))
	suite.fakeAuth = fake_auth.New(sessionStore)
	w := testingLogWriter{suite: &suite.Suite}
	output := zerolog.ConsoleWriter{Out: &w, NoColor: true}
	logger := zerolog.New(output)

	suite.server = tabloid.NewServer(config, logger, suite.pgStore, suite.fakeAuth)
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

func (suite *IntegrationTestSuite) TestAuthentication() {
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

	assert.True(t, strings.Contains(string(body), "fakeLogin"))
}

func (suite *IntegrationTestSuite) TestSubmitComment() {
	t := suite.T()

	// enable cookies on the client side for authentication
	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}

	// create a story to comment on
	err := suite.pgStore.InsertStory(&tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Score:     1,
		Body:      "Foobaring",
		AuthorID:  suite.existingUserID,
		CreatedAt: time.Now(),
	})
	assert.NoError(t, err)

	// authenticate
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	assert.NoError(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// find the link to the story
	storyRegexp := regexp.MustCompile(`(/stories/\d+/comments)`)
	path := storyRegexp.FindString(string(body))
	assert.NotEmpty(t, path, "Story path was found empty")
	storyUrl := suite.testServer.URL + path

	// get that page
	resp, err = client.Get(storyUrl)
	assert.NoError(t, err)

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// submit a comment
	values := url.Values{
		"body":      []string{"very insightful comment"},
		"parent-id": []string{""},
	}

	resp, err = client.PostForm(storyUrl, values)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	assert.True(t, strings.Contains(string(body), "very insightful comment"))
	// initial score is always 1
	assert.True(t, strings.Contains(string(body), "1 by alpha, today"))

	// submit a subcomment

	// get the form hidden input
	// this should probably get refactored into something more automated and robust as we add more forms to the app
	parentCommentRegexp := regexp.MustCompile(`<input type="hidden" name="parent-id" value="(\d+)">`)
	matches := parentCommentRegexp.FindStringSubmatch(string(body))
	assert.Len(t, matches, 2, "could not find parent comment id hidden input")
	parentCommentID := matches[1]
	assert.NotEmpty(t, parentCommentID, "Parent comment id was found empty")

	// submit a subcomment
	values = url.Values{
		"body":      []string{"quite logical subcomment"},
		"parent-id": []string{parentCommentID},
	}

	resp, err = client.PostForm(storyUrl, values)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	assert.True(t, strings.Contains(string(body), `quite logical subcomment`))
}

func (suite *IntegrationTestSuite) TestPagination() {
	t := suite.T()

	// create the stories
	for i := 0; i < 20; i++ {
		ii := strconv.Itoa(i)
		err := suite.pgStore.InsertStory(&tabloid.Story{
			Title:     "Foobar" + ii,
			URL:       "http://foobar.com/" + ii,
			Score:     1,
			Body:      "Foobaring",
			AuthorID:  suite.existingUserID,
			CreatedAt: time.Now(),
		})
		assert.NoError(t, err)
	}

	// get the homepage
	resp, err := http.Get(suite.testServer.URL)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	// check that we get the latest stories
	assert.True(t, strings.Contains(string(body), "http://foobar.com/19"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/18"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/17"))
	assert.False(t, strings.Contains(string(body), "http://foobar.com/3"))

	// check that there is no prev link on the first page
	paginationLink := regexp.MustCompile(`<a href="(/?page=\d+)">Prev</a>`)
	path := paginationLink.FindString(string(body))
	assert.Empty(t, path)

	// click the next link
	paginationLink = regexp.MustCompile(`<a href="(/\?page=\d+)">Next</a>`)
	matches := paginationLink.FindStringSubmatch(string(body))
	assert.Equal(t, 2, len(matches))
	path = matches[1]
	assert.NotEmpty(t, path, "Pagination link not found")
	nextPage := suite.testServer.URL + path

	resp, err = http.Get(nextPage)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	// check that we get the next page of stories
	assert.True(t, strings.Contains(string(body), "http://foobar.com/16"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/15"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/14"))
	assert.False(t, strings.Contains(string(body), "http://foobar.com/19"))

	// click the prev link
	paginationLink = regexp.MustCompile(`<a href="(/\?page=\d+)">Prev</a>`)
	matches = paginationLink.FindStringSubmatch(string(body))
	assert.Equal(t, 2, len(matches))
	path = matches[1]
	assert.NotEmpty(t, path, "Pagination link not found")
	nextPage = suite.testServer.URL + path

	resp, err = http.Get(nextPage)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.Nil(t, err)

	// check that we are back on the homepage
	assert.True(t, strings.Contains(string(body), "http://foobar.com/19"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/18"))
	assert.True(t, strings.Contains(string(body), "http://foobar.com/17"))
	assert.False(t, strings.Contains(string(body), "http://foobar.com/16"))
}
