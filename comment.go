package tabloid

import "time"

type Comment struct {
	ID              int64     `db:"id"`
	ParentCommentID int64     `db:"parent_comment_id"`
	StoryID         int64     `db:"story_id"`
	Body            string    `db:"body"`
	Upvotes         int64     `db:"upvotes"`
	Downvotes       int64     `db:"downvotes"`
	Author          string    `db:"author"`
	CreatedAt       time.Time `db:"created_at"`
}

func NewComment(storyID int64, parentCommentID int64, body string, author string) *Comment {
	return &Comment{
		ParentCommentID: parentCommentID,
		StoryID:         storyID,
		Body:            body,
		Author:          author,
		CreatedAt:       time.Now(),
	}
}
