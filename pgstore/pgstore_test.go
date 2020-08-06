package pgstore

import (
	"github.com/jhchabran/tabloid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PGStoreTestSuite struct {
	suite.Suite
	pgStore *PGStore
}

func (suite *PGStoreTestSuite) SetupTest() {
	suite.pgStore = New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	require.NoError(suite.T(), suite.pgStore.Connect())
}

func (suite *PGStoreTestSuite) TearDownTest() {
	suite.pgStore.DB().MustExec("TRUNCATE TABLE stories;")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE comments;")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE users;")
	suite.pgStore.DB().MustExec("TRUNCATE TABLE votes;")
}

func (suite *PGStoreTestSuite) TestInsertStory() {
	r := require.New(suite.T())
	store := New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	r.NoError(store.Connect())

	var userID int64 = 1
	story := tabloid.NewStory("foo", "body", userID, "http://foobar.com")
	err := store.InsertStory(story)
	r.NoError(err)
	r.NotZero(story.ID)

	vote := &tabloid.Vote{}
	err = suite.pgStore.db.Get(&vote, "SELECT * FROM votes WHERE story_id = $1 AND user_id = 1 LIMIT 1", story.ID)
	r.NoError(err)

	r.Equal(userID, vote.UserID)
	r.Equal(story.ID, vote.StoryID)
	r.Truef(vote.Up, "vote must be up after creating a story")

	r.Equal(story.Score, 1, "story must have its score field updated")
}
