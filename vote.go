package tabloid

import "time"

type Vote struct {
	ID        int64     `db:"id"`
	CommentID int64     `db:"comment_id"`
	StoryID   int64     `db:"story_id"`
	UserID    int64     `db:"user_id"`
	Up        bool      `db:"up"`
	CreatedAt time.Time `db:"created_at"`
}
