package tabloid

import "time"

type Comment struct {
	ID        int       `db:"id"`
	ParentID  int       `db:"parent_id"`
	Body      string    `db:"body"`
	Score     int       `db:"score"`
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
