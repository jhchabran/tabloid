package pgstore

import (
	"database/sql"
	"strconv"
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

	c.Run("List comments with votes", func(c *qt.C) {
		c.Cleanup(func() {
			store.DB().MustExec("TRUNCATE TABLE stories;")
			store.DB().MustExec("TRUNCATE TABLE comments;")
			store.DB().MustExec("TRUNCATE TABLE users;")
			store.DB().MustExec("TRUNCATE TABLE votes;")
		})

		userA, err := store.CreateOrUpdateUser("a", "a@a.com")
		c.Assert(err, qt.IsNil)
		userB, err := store.CreateOrUpdateUser("b", "b@b.com")
		c.Assert(err, qt.IsNil)

		// create a story that we'll use to test the function
		story := tabloid.NewStory("foo", "body", userA, "http://foobar.com")
		err = store.InsertStory(story)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), story.ID)

		comment := tabloid.NewComment(story.ID, sql.NullInt64{}, "foobar", userB)
		err = store.InsertComment(comment)
		c.Assert(err, qt.IsNil)
		err = store.CreateOrUpdateVoteOnComment(comment.ID, userA, true)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), comment.ID)

		// create another story, whose comments should not appear in the results
		otherStory := tabloid.NewStory("foo", "body", userA, "http://foobar.com")
		err = store.InsertStory(otherStory)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), otherStory.ID)

		otherComment := tabloid.NewComment(otherStory.ID, sql.NullInt64{}, "other foobar", userB)
		err = store.InsertComment(otherComment)
		c.Assert(err, qt.IsNil)
		err = store.CreateOrUpdateVoteOnComment(otherComment.ID, userA, true)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), otherComment.ID)

		// check what the function returns
		commentsWithVotes, err := store.ListCommentsWithVotes(strconv.Itoa(int(story.ID)), userB)
		c.Assert(err, qt.IsNil)

		c.Assert(commentsWithVotes, qt.HasLen, 1)
		c.Assert(commentsWithVotes[0].StoryID, qt.Equals, story.ID)
		c.Assert(commentsWithVotes[0].UserID, qt.Equals, userB)
		c.Assert(commentsWithVotes[0].Up, qt.Equals, sql.NullBool{Valid: true, Bool: true})
	})
}
