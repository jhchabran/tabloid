package tabloid

import (
	"database/sql"
	"time"
)

type Vote struct {
	ID        string         `db:"id"`
	CommentID sql.NullString `db:"comment_id"`
	StoryID   sql.NullString `db:"story_id"`
	UserID    string         `db:"user_id"`
	Up        bool           `db:"up"`
	CreatedAt time.Time      `db:"created_at"`
}
