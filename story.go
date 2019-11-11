package tabloid

import "time"

type Story struct {
	ID        int64     `db:"id"`
	Title     string    `db:"title"`
	URL       string    `db:"url"`
	Body      string    `db:"body"`
	Score     int64     `db:"score"`
	Author    string    `db:"author"`
	CreatedAt time.Time `db:"created_at"`
}

func NewStory(title string, body string, author string) *Story {
	return &Story{
		Title:     title,
		Body:      body,
		Score:     1,
		Author:    author,
		CreatedAt: time.Now(),
	}
}
