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

		var userID = "1"
		story := tabloid.NewStory("foo", "body", userID, "http://foobar.com")
		err := store.InsertStory(story)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), story.ID)

		vote := &tabloid.Vote{}
		err = store.db.Get(vote, "SELECT * FROM votes WHERE story_id = $1 AND user_id = 1 LIMIT 1", story.ID)

		c.Assert(err, qt.IsNil)
		c.Assert(vote.UserID, qt.Equals, userID)
		c.Assert(vote.StoryID, qt.Equals, sql.NullString{String: story.ID, Valid: true})
		c.Assert(vote.Up, qt.IsTrue, qt.Commentf("vote must be up when creating a story"))
		c.Assert(story.Score, qt.Equals, 1, qt.Commentf("story must have its score field updated"))
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

		comment := tabloid.NewComment(story.ID, sql.NullString{}, "foobar", userB)
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

		otherComment := tabloid.NewComment(otherStory.ID, sql.NullString{}, "other foobar", userB)
		err = store.InsertComment(otherComment)
		c.Assert(err, qt.IsNil)
		err = store.CreateOrUpdateVoteOnComment(otherComment.ID, userA, true)
		c.Assert(err, qt.IsNil)
		c.Assert(0, qt.Not(qt.Equals), otherComment.ID)

		// check what the function returns
		commentsWithVotes, err := store.ListCommentsWithVotes(story.ID, userB)
		c.Assert(err, qt.IsNil)

		c.Assert(commentsWithVotes, qt.HasLen, 1)
		c.Assert(commentsWithVotes[0].StoryID, qt.Equals, story.ID)
		c.Assert(commentsWithVotes[0].UserID, qt.Equals, userB)
		c.Assert(commentsWithVotes[0].Up, qt.Equals, sql.NullBool{Valid: true, Bool: true})
	})

	c.Run("Find non-existing user", func(c *qt.C) {
		userRecord, err := store.FindUserByLogin("non-existing")
		c.Assert(err, qt.IsNil)
		c.Assert(userRecord, qt.IsNil)
	})

	c.Run("Updating a comment", func(c *qt.C) {
		c.Run("OK", func(c *qt.C) {
			c.Cleanup(func() {
				store.DB().MustExec("TRUNCATE TABLE stories;")
				store.DB().MustExec("TRUNCATE TABLE comments;")
				store.DB().MustExec("TRUNCATE TABLE users;")
				store.DB().MustExec("TRUNCATE TABLE votes;")
			})

			comment := tabloid.NewComment("1", sql.NullString{String: "Foo"}, "foobar", "1")
			err := store.InsertComment(comment)
			c.Assert(err, qt.IsNil)

			comment.Body = "Bar"
			err = store.UpdateComment(comment)
			c.Assert(err, qt.IsNil)
		})

		c.Run("non existing comment", func(c *qt.C) {
			comment := tabloid.NewComment("1", sql.NullString{String: "Foo"}, "foobar", "1")
			comment.ID = "666"
			err := store.UpdateComment(comment)
			c.Assert(err, qt.Equals, recordNotFoundError)
		})
	})

	c.Run("Getting a user", func(c *qt.C) {
		c.Cleanup(func() {
			store.DB().MustExec("TRUNCATE TABLE users;")
		})

		store.DB().MustExec("INSERT INTO users (name, email, settings, created_at, last_login_at) VALUES ($1, $2, $3, $4, $5)",
			"foobar",
			"foobar@foobar.com",
			tabloid.UserSettings{SendDailyDigest: true},
			tabloid.NowFunc(),
			tabloid.NowFunc())

		c.Run("OK", func(c *qt.C) {
			user, err := store.FindUserByLogin("foobar")
			c.Assert(err, qt.IsNil)
			c.Assert(user, qt.Not(qt.IsNil))
		})

		c.Run("OK Settings", func(c *qt.C) {
			user, err := store.FindUserByLogin("foobar")
			c.Assert(err, qt.IsNil)
			c.Assert(user.Settings.SendDailyDigest, qt.IsTrue)
		})
	})

	c.Run("Updating a user", func(c *qt.C) {
		c.Cleanup(func() {
			store.DB().MustExec("TRUNCATE TABLE users;")
		})

		store.DB().MustExec("INSERT INTO users (name, email, settings, created_at, last_login_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			"foobar",
			"foobar@foobar.com",
			tabloid.UserSettings{SendDailyDigest: true},
			tabloid.NowFunc(),
			tabloid.NowFunc(),
		)

		c.Run("OK", func(c *qt.C) {
			user, err := store.FindUserByLogin("foobar")
			c.Assert(err, qt.IsNil)

			// changing some setting and email
			user.Settings.SendDailyDigest = false
			user.Email = "barfoo@foobar.com"

			err = store.UpdateUser(user)
			c.Assert(err, qt.IsNil)

			// load the user from db again
			user, err = store.FindUserByLogin("foobar")
			c.Assert(err, qt.IsNil)

			c.Assert(user.Email, qt.Equals, "barfoo@foobar.com")
			c.Assert(user.Settings.SendDailyDigest, qt.IsFalse)
		})

	})
}
