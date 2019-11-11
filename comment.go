package tabloid

import "time"

type Comment struct {
	ID        int64     `db:"id"`
	ParentID  int64     `db:"parent_id"`
	Body      string    `db:"body"`
	Score     int64     `db:"score"`
	Author    string    `db:"author"`
	CreatedAt time.Time `db:"created_at"`
}

func NewComment(parentID int, body string, author string) *Comment {
	return &Comment{
		Body:      body,
		Author:    author,
		Score:     1,
		CreatedAt: time.Now(),
	}
}
