package tabloid

import (
	"database/sql"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewCommentOK(t *testing.T) {
	c := qt.New(t)

	comment := NewComment(1, sql.NullInt64{Int64: 1, Valid: true}, "body", 1)
	c.Assert(int64(1), qt.Equals, comment.ParentCommentID.Int64)
	c.Assert(comment.ParentCommentID.Valid, qt.IsTrue)
}

func TestNewCommentPresentersTree(t *testing.T) {
	c := qt.New(t)

	authorID := int64(42)
	nilParent := sql.NullInt64{Int64: 0, Valid: false}
	a := NewComment(1, nilParent, "body", authorID)
	a.ID = 1
	a1 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", authorID)
	a1.ID = 2
	a2 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", authorID)
	a2.ID = 3
	a3 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", authorID)
	a3.ID = 4
	a21 := NewComment(int64(1), sql.NullInt64{Int64: a2.ID, Valid: true}, "", authorID)
	a21.ID = 5
	a22 := NewComment(int64(1), sql.NullInt64{Int64: a2.ID, Valid: true}, "", authorID)
	a22.ID = 6
	b := NewComment(1, nilParent, "body", authorID)
	b.ID = 7
	b1 := NewComment(int64(1), sql.NullInt64{Int64: b.ID, Valid: true}, "", authorID)
	b1.ID = 8

	comments := []CommentAccessor{
		a,
		a1,
		a2,
		a3,
		a21,
		a22,
		b,
		b1,
	}

	ps := NewCommentPresentersTree(comments)

	tests := map[int]int64{
		1: ps[0].ID,
		2: ps[0].Children[0].ID,
		3: ps[0].Children[1].ID,
		4: ps[0].Children[2].ID,
		5: ps[0].Children[1].Children[0].ID,
		6: ps[0].Children[1].Children[1].ID,
		7: ps[1].ID,
		8: ps[1].Children[0].ID,
	}

	for want, input := range tests {
		c.Assert(int64(want), qt.Equals, input)
	}
}
