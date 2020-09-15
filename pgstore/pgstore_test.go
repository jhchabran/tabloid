package pgstore

import (
	"database/sql"
	"testing"

	"github.com/jhchabran/tabloid"

	qt "github.com/frankban/quicktest"
)

func TestPGStore(t *testing.T) {
	c := qt.New(t)
	store := New("user=postgres dbname=tabloid_test sslmode=disable password=postgres host=127.0.0.1")
	c.Assert(store.Connect(), qt.IsNil)

	c.Run("InsertStory", func(c *qt.C) {
		c.Cleanup(func() {
			store.DB().MustExec("TRUNCATE TABLE stories;")
			store.DB().MustExec("TRUNCATE TABLE comments;")
			store.DB().MustExec("TRUNCATE TABLE users;")
			store.DB().MustExec("TRUNCATE TABLE votes;")
		})

		var userID int64 = 1
		story := tabloid.NewStory("foo", "body", userID, "http://foobar.com")
		err := store.InsertStory(story)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), story.ID)

		vote := &tabloid.Vote{}
		err = store.db.Get(vote, "SELECT * FROM votes WHERE story_id = $1 AND user_id = 1 LIMIT 1", story.ID)

		c.Assert(err, qt.IsNil)
		c.Assert(vote.UserID, qt.Equals, userID)
		c.Assert(vote.StoryID, qt.Equals, sql.NullInt64{Int64: story.ID, Valid: true})
		c.Assert(vote.Up, qt.IsTrue, qt.Commentf("vote must be up when creating a story"))
		c.Assert(story.Score, qt.Equals, int64(1), qt.Commentf("story must have its score field updated"))
	})
}
