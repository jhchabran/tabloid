package tabloid

import (
	"database/sql"
	"time"
)

type Vote struct {
	ID        int64         `db:"id"`
	CommentID sql.NullInt64 `db:"comment_id"`
	StoryID   sql.NullInt64 `db:"story_id"`
	UserID    int64         `db:"user_id"`
	Up        bool          `db:"up"`
	CreatedAt time.Time     `db:"created_at"`
}
