package integration

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	suite.Suite
	pgStore *pgstore.PGStore
}

func (suite *IntegrationTestSuite) SetupTest() {
	err := os.Chdir("..")
	if err != nil {
		suite.FailNow("%v", err)
	}
	suite.pgStore = pgstore.New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
}

func (suite *IntegrationTestSuite) TearDownTest() {
	os.Chdir("integration")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (suite *IntegrationTestSuite) TestEmptyIndex() {
	t := suite.T()

	s := tabloid.NewServer("localhost:8081", suite.pgStore)
	assert.Nil(t, s.Prepare())

	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	assert.Nil(t, err)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
	}
}

func (suite *IntegrationTestSuite) TestIndexWithStory() {
	t := suite.T()
	s := tabloid.NewServer("localhost:8081", suite.pgStore)
	assert.Nil(t, s.Prepare())

	ts := httptest.NewServer(s)
	defer ts.Close()

	err := suite.pgStore.InsertStory(&tabloid.Story{
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
