package integration

import (
	"database/sql"
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

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/fake_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
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
	w := testingLogWriter{suite: &suite.Suite}
	output := zerolog.ConsoleWriter{Out: &w, NoColor: true}
	logger := zerolog.New(output)
	suite.pgStore = pgstore.New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	sessionStore := sessions.NewCookieStore([]byte("test"))
	suite.fakeAuth = fake_auth.New(sessionStore, logger)

	suite.server = tabloid.NewServer(config, logger, suite.pgStore, suite.fakeAuth)
	suite.testServer = httptest.NewServer(suite.server)
	suite.fakeAuth.SetServerURL(suite.testServer.URL)

	require.Nil(suite.T(), suite.server.Prepare())
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
	suite.pgStore.DB().MustExec("TRUNCATE TABLE votes;")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (suite *IntegrationTestSuite) TestEmptyIndex() {
	t := suite.T()

	resp, err := http.Get(suite.testServer.URL)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
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
	require.NoError(t, err)

	resp, err := http.Get(suite.testServer.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	require.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	require.True(t, strings.Contains(string(body), "Foobar"))
	require.True(t, strings.Contains(string(body), "http://foobar.com"))
}

func (suite *IntegrationTestSuite) TestSubmitStory() {
	t := suite.T()

	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// test for the form being rendered
	resp, err = client.Get(suite.testServer.URL + "/submit")
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	require.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	require.True(t, strings.Contains(string(body), "id=\"submit-form\""))

	// test for submitting the form
	values := url.Values{
		"title": []string{"Captain Nemo"},
		"url":   []string{"http://duckduckgo.com"},
		"body":  []string{"foobar"},
	}
	resp, err = client.PostForm(suite.testServer.URL+"/submit", values)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	require.True(t, strings.Contains(string(body), "<title>Tabloid</title>"))
	require.True(t, strings.Contains(string(body), "Captain Nemo"))
	require.True(t, strings.Contains(string(body), "href=\"http://duckduckgo.com\""))
}

func (suite *IntegrationTestSuite) TestAuthentication() {
	t := suite.T()

	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	require.True(t, strings.Contains(string(body), "fakeLogin"))
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
	require.NoError(t, err)

	// authenticate
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	require.NoError(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// require that comment count is set to zero and correctly pluralized on the homepage
	require.True(t, strings.Contains(string(body), "0 Comments"))

	// find the link to the story
	storyRegexp := regexp.MustCompile(`(/stories/\d+/comments)`)
	path := storyRegexp.FindString(string(body))
	require.NotEmpty(t, path, "Story path was found empty")
	storyUrl := suite.testServer.URL + path

	// get that page
	resp, err = client.Get(storyUrl)
	require.NoError(t, err)

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// submit a comment
	values := url.Values{
		"body":      []string{"very insightful comment"},
		"parent-id": []string{""},
	}

	resp, err = client.PostForm(storyUrl, values)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	require.True(t, strings.Contains(string(body), "very insightful comment"))

	// initial score is always 1
	require.Contains(t, string(body), "1 by alpha, today")

	// ensure that comments get pluralized properly on the homepage
	resp2, err := client.Get(suite.testServer.URL)
	body2, err := ioutil.ReadAll(resp2.Body)
	defer resp2.Body.Close()
	require.Nil(t, err)
	require.True(t, strings.Contains(string(body2), "1 Comment"))

	// submit a subcomment
	// get the form hidden input
	// this should probably get refactored into something more automated and robust as we add more forms to the app
	parentCommentRegexp := regexp.MustCompile(`<input type="hidden" name="parent-id" value="(\d+)">`)
	matches := parentCommentRegexp.FindAllStringSubmatch(string(body), -1)
	require.Len(t, matches, 1, "could not find parent comment id hidden input")
	parentCommentID := matches[0][1]
	require.NotEmpty(t, parentCommentID, "Parent comment id was found empty")

	// submit a subcomment
	values = url.Values{
		"body":      []string{"quite logical subcomment"},
		"parent-id": []string{parentCommentID},
	}

	resp, err = client.PostForm(storyUrl, values)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// ensure that the comment body is present
	require.True(t, strings.Contains(string(body), `quite logical subcomment`))

	// ensure that comments get pluralized properly
	resp2, err = client.Get(suite.testServer.URL + "/")
	body2, err = ioutil.ReadAll(resp2.Body)
	defer resp2.Body.Close()
	require.Nil(t, err)
	require.True(t, strings.Contains(string(body2), "2 Comments"))
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
		require.NoError(t, err)
	}

	// get the homepage
	resp, err := http.Get(suite.testServer.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	// check that we get the latest stories
	require.True(t, strings.Contains(string(body), "http://foobar.com/19"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/18"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/17"))
	require.False(t, strings.Contains(string(body), "http://foobar.com/3"))

	// check that there is no prev link on the first page
	paginationLink := regexp.MustCompile(`<a href="(/?page=\d+)">Prev</a>`)
	path := paginationLink.FindString(string(body))
	require.Empty(t, path)

	// click the next link
	paginationLink = regexp.MustCompile(`<a href="(/\?page=\d+)">Next</a>`)
	matches := paginationLink.FindStringSubmatch(string(body))
	require.Equal(t, 2, len(matches))
	path = matches[1]
	require.NotEmpty(t, path, "Pagination link not found")
	nextPage := suite.testServer.URL + path

	resp, err = http.Get(nextPage)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	// check that we get the next page of stories
	require.True(t, strings.Contains(string(body), "http://foobar.com/16"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/15"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/14"))
	require.False(t, strings.Contains(string(body), "http://foobar.com/19"))

	// click the prev link
	paginationLink = regexp.MustCompile(`<a href="(/\?page=\d+)">Prev</a>`)
	matches = paginationLink.FindStringSubmatch(string(body))
	require.Equal(t, 2, len(matches))
	path = matches[1]
	require.NotEmpty(t, path, "Pagination link not found")
	nextPage = suite.testServer.URL + path

	resp, err = http.Get(nextPage)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	require.Nil(t, err)

	// check that we are back on the homepage
	require.True(t, strings.Contains(string(body), "http://foobar.com/19"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/18"))
	require.True(t, strings.Contains(string(body), "http://foobar.com/17"))
	require.False(t, strings.Contains(string(body), "http://foobar.com/16"))
}

func (suite *IntegrationTestSuite) TestVotingOnStories() {
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
		Body:      "Foobaring",
		AuthorID:  suite.existingUserID,
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	// authenticate
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	require.NoError(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// Find the upvote button
	doc, err := goquery.NewDocumentFromReader(resp.Body)

	require.Nil(t, err)
	action, ok := doc.Find(".voters form.upvoter").Attr("action")
	require.True(t, ok)
	require.NotNil(t, action)

	// referer is used to send back to the page where the vote was done, ie the index
	req, err := http.NewRequest("POST", suite.testServer.URL+action, nil)
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", suite.testServer.URL)
	resp, err = client.Do(req)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()
	doc, err = goquery.NewDocumentFromReader(resp.Body)

	// The story score should be 2 (original upvote plus this one)
	require.Contains(t, doc.Find("span.story-meta").Text(), "2 by alpha, today")

	// There shouldn't be an upvote button anymore for this user
	_, ok = doc.Find(".voters form.upvoter button").Attr("disabled")
	require.Truef(t, ok, "disabled attribute must be present on the button")

	// Log out and upvote with a different user
	resp, err = client.Get(suite.testServer.URL + "/oauth/destroy")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	// Check that the upvote button is present and sends to login for unathenticated users
	defer resp.Body.Close()
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	href, ok := doc.Find("a.voters-inactive").Attr("href")
	require.Truef(t, ok, "cannot find placeholder for unathenticated upvotes")
	require.Equal(t, href, "/oauth/start")

	// Login with a different user, the fake_auth package will create a new one for each subsequent login
	resp, err = client.Get(suite.testServer.URL + "/oauth/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	// Upvote again
	doc, err = goquery.NewDocumentFromReader(resp.Body)

	require.Nil(t, err)
	action, ok = doc.Find(".voters form.upvoter").Attr("action")
	require.True(t, ok)
	require.NotNil(t, action)

	// referer is used to send back to the page where the vote was done, ie the index
	req, err = http.NewRequest("POST", suite.testServer.URL+action, nil)
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", suite.testServer.URL)
	resp, err = client.Do(req)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// The story score should be now be 3
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	require.Contains(t, doc.Find("span.story-meta").Text(), "3 by alpha, today")

	// There shouldn't be an upvote button anymore for this second user
	require.Empty(t, doc.Find("a.voters-inactive").Nodes)
}

func (suite *IntegrationTestSuite) TestVotingOnComments() {
	t := suite.T()

	// enable cookies on the client side for authentication
	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: cookieJar,
	}

	// create a story to comment on
	story := tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Body:      "Foobaring",
		AuthorID:  suite.existingUserID,
		CreatedAt: time.Now(),
	}
	err := suite.pgStore.InsertStory(&story)
	require.NoError(t, err)

	// create a root comment
	comment := tabloid.NewComment(story.ID, sql.NullInt64{}, "foobar", suite.existingUserID)
	err = suite.pgStore.InsertComment(comment)
	require.NoError(t, err)

	// authenticate
	resp, err := client.Get(suite.testServer.URL + "/oauth/start")
	require.NoError(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// navigate to the story page
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	path, ok := doc.Find("a.story-comments").Attr("href")
	require.Truef(t, true, "Cannot find link to story comments")
	resp, err = client.Get(suite.testServer.URL + path)
	require.NoError(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// Find the upvote button
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	require.Nil(t, err)

	action, ok := doc.Find(".voters form.upvoter").Attr("action")
	require.True(t, ok)
	require.NotNil(t, action)

	// referer is used to send back to the page where the vote was done, ie the story page
	req, err := http.NewRequest("POST", suite.testServer.URL + path, nil)
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", suite.testServer.URL + path)
	resp, err = client.Do(req)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()
	doc, err = goquery.NewDocumentFromReader(resp.Body)

	// The story score should be 2 (original upvote plus this one)
	require.Contains(t, doc.Find("span.comment-meta").Text(), "alpha, 2 points, today")

	// There shouldn't be an upvote button anymore for this user
	_, ok = doc.Find(".voters form.upvoter button").Attr("disabled")
	require.Truef(t, ok, "disabled attribute must be present on the button")

	// Log out and upvote with a different user
	resp, err = client.Get(suite.testServer.URL + "/oauth/destroy")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	// Check that the upvote button is present and sends to login for unathenticated users
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	href, ok := doc.Find("a.voters-inactive").Attr("href")
	require.Truef(t, ok, "cannot find placeholder for unathenticated upvotes")
	require.Equal(t, href, "/oauth/start")

	// Login with a different user, the fake_auth package will create a new one for each subsequent login
	resp, err = client.Get(suite.testServer.URL + "/oauth/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotNil(t, resp)
	require.Equal(t, 200, resp.StatusCode)

	// navigate to the story page
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	path, ok = doc.Find("a.story-comments").Attr("href")
	require.Truef(t, true, "Cannot find link to story comments")
	resp, err = client.Get(suite.testServer.URL + path)
	require.NoError(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// Upvote again
	doc, err = goquery.NewDocumentFromReader(resp.Body)

	require.Nil(t, err)
	action, ok = doc.Find(".voters form.upvoter").Attr("action")
	require.True(t, ok)
	require.NotNil(t, action)

	// referer is used to send back to the page where the vote was done, ie the index
	req, err = http.NewRequest("POST", suite.testServer.URL+action, nil)
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", suite.testServer.URL + path)
	resp, err = client.Do(req)
	require.Nil(t, err)
	if resp != nil {
		require.Equal(t, 200, resp.StatusCode)
	}
	defer resp.Body.Close()

	// The story score should be now be 3
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	require.Contains(t, doc.Find("span.comment-meta").Text(), "alpha, 3 points, today")

	// There shouldn't be an upvote button anymore for this second user
	require.Empty(t, doc.Find("a.voters-inactive").Nodes)
}
