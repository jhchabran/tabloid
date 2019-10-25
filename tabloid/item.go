package tabloid

import "time"

type Item struct {
	ID        int       `db:"id"`
	Title     string    `db:"title"`
	Body      string    `db:"body"`
	Score     int       `db:"score"`
	Author    string    `db:"author"`
	CreatedAt time.Time `db:"created_at"`
}

func NewItem(title string, body string, author string) *Item {
	return &Item{
		Title:     title,
		Body:      body,
		Score:     1,
		Author:    author,
		CreatedAt: time.Now(),
	}
}
