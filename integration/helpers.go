package integration

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"

	qt "github.com/frankban/quicktest"
	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication/fake_auth"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

const (
	dbString       = "user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1"
	testServerHost = "localhost:8081"
)

func truncateDatabase(db *sqlx.DB) {
	db.MustExec("TRUNCATE TABLE stories;")
	db.MustExec("TRUNCATE TABLE comments;")
	db.MustExec("TRUNCATE TABLE users;")
	db.MustExec("TRUNCATE TABLE votes;")
}

// testingLogWriter is an output target for zerolog which will print on the testing logger.
type testingLogWriter struct {
	c *qt.C
}

// Write outputs on the passed bytes on the test logger
func (l *testingLogWriter) Write(p []byte) (n int, err error) {
	str := string(p[0 : len(p)-1]) // drop the final \n
	l.c.Log(str)
	return len(p), nil
}

// A struct to hold the server and its components.
// Provides a few helpers for convenience.
type testContext struct {
	c          *qt.C
	server     *tabloid.Server
	testServer *httptest.Server
	pgStore    *pgstore.PGStore
}

// newTestContext creates a server instance with its component initialized for integration testing.
func newTestContext(c *qt.C) *testContext {
	tc := testContext{c: c}

	w := testingLogWriter{c}
	output := zerolog.ConsoleWriter{Out: &w, NoColor: true}
	logger := zerolog.New(output)

	tc.pgStore = pgstore.New(dbString)
	sessionStore := sessions.NewCookieStore([]byte("test"))
	fakeAuth := fake_auth.New(sessionStore, logger)

	tc.server = tabloid.NewServer(
		&tabloid.ServerConfig{Addr: testServerHost, StoriesPerPage: 3},
		logger,
		tc.pgStore,
		fakeAuth,
	)
	tc.testServer = httptest.NewServer(tc.server)

	fakeAuth.SetServerURL(tc.testServer.URL)

	return &tc
}

// url returns an url to the test server based on the given path
func (tc *testContext) url(path string) string {
	return tc.testServer.URL + path
}

// prepareServer boots up the server and sets up its teardown for the current test
func (tc *testContext) prepareServer() {
	// move the right directory for the templates
	err := os.Chdir("..")
	if err != nil {
		tc.c.Fatalf("%v", err)
	}

	tc.c.Assert(tc.server.Prepare(), qt.IsNil, qt.Commentf("couldn't prepare the server"))
	tc.c.Cleanup(func() {
		// kill the server
		tc.testServer.Close()

		// restore the db to its pristine state
		truncateDatabase(tc.pgStore.DB())

		// chdir back to the right cwd
		err := os.Chdir("integration")
		if err != nil {
			tc.c.Fatalf("%v", err)
		}
	})
}

func (tc *testContext) createUser(login string) (int64, error) {
	var id int64
	t := tabloid.NowFunc()
	err := tc.pgStore.DB().Get(&id,
		"INSERT INTO users (name, email, created_at, last_login_at) VALUES ($1, $2, $3, $4) RETURNING id",
		login, login+"@email.com", t, t)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (tc *testContext) newHTTPClient() *http.Client {
	jar, err := cookiejar.New(nil)
	tc.c.Assert(err, qt.IsNil)

	return &http.Client{
		Jar: jar,
	}
}

func (tc *testContext) newAuthenticatedClient() *http.Client {
	client := tc.newHTTPClient()
	resp, err := client.Get(tc.url("/oauth/start"))
	tc.c.Assert(err, qt.IsNil)
	defer resp.Body.Close()
	tc.c.Assert(resp.StatusCode, qt.Equals, 200)
	return client
}
