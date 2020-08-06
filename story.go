package tabloid

import (
	"database/sql"
	"time"
)

type Story struct {
	ID            int64     `db:"id"`
	Title         string    `db:"title"`
	URL           string    `db:"url"`
	Body          string    `db:"body"`
	Score         int64     `db:"score"`
	Author        string    `db:"author"`
	AuthorID      int64     `db:"author_id"`
	CommentsCount int64     `db:"comments_count"`
	CreatedAt     time.Time `db:"created_at"`
}

type StorySeenByUser struct {
	Story
	UserId int64        `db:"user_id"`
	Up     sql.NullBool `db:"up"`
}

func NewStory(title string, body string, authorID int64, url string) *Story {
	return &Story{
		Title:     title,
		Body:      body,
		Score:     0,
		AuthorID:  authorID,
		URL:       url,
		CreatedAt: NowFunc(),
	}
}
