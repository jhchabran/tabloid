package tabloid

import (
	"database/sql"
	"time"
)

type Comment struct {
	ID              int64         `db:"id"`
	ParentCommentID sql.NullInt64 `db:"parent_comment_id"`
	StoryID         int64         `db:"story_id"`
	Body            string        `db:"body"`
	Upvotes         int64         `db:"upvotes"`
	Downvotes       int64         `db:"downvotes"`
	Author          string        `db:"author"`
	CreatedAt       time.Time     `db:"created_at"`
}

// TODO move this in a better place
type CommentPresenter struct {
	Path       string
	ParentPath string
	StoryPath  string
	Body       string
	Score      int64
	Author     string
	CreatedAt  time.Time
}

func NewComment(storyID int64, parentCommentID int64, body string, author string) *Comment {
	id := sql.NullInt64{
		Int64: parentCommentID,
		Valid: true,
	}
	return &Comment{
		ParentCommentID: id,
		StoryID:         storyID,
		Body:            body,
		Author:          author,
		CreatedAt:       time.Now(),
	}
}

func (c *Comment) Score() int64 {
	return c.Upvotes - c.Downvotes
}

// TODO missing fields
func NewCommentPresenter(c *Comment) *CommentPresenter {
	return &CommentPresenter{
		Body:   c.Body,
		Score:  c.Score(),
		Author: c.Author,
	}
}
