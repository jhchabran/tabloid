package tabloid

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCommentOK(t *testing.T) {
	comment := NewComment(1, sql.NullInt64{Int64: 1, Valid: true}, "body", "author")
	require.Equal(t, int64(1), comment.ParentCommentID.Int64)
	require.True(t, comment.ParentCommentID.Valid)
}

func TestNewCommentTree(t *testing.T) {
	nilParent := sql.NullInt64{Int64: 0, Valid: false}
	a := NewComment(1, nilParent, "body", "author")
	a.ID = 1
	a1 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", "")
	a1.ID = 2
	a2 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", "")
	a2.ID = 3
	a3 := NewComment(int64(1), sql.NullInt64{Int64: a.ID, Valid: true}, "", "")
	a3.ID = 4
	a21 := NewComment(int64(1), sql.NullInt64{Int64: a2.ID, Valid: true}, "", "")
	a21.ID = 5
	a22 := NewComment(int64(1), sql.NullInt64{Int64: a2.ID, Valid: true}, "", "")
	a22.ID = 6

	comments := []*Comment{
		a,
		a1,
		a2,
		a3,
		a21,
		a22,
	}

	tree := NewCommentTree(comments)

	require.Equal(t, int64(1), tree.Comment.ID)
	require.Equal(t, int64(2), tree.Children[0].Comment.ID)
	require.Equal(t, int64(3), tree.Children[1].Comment.ID)
	require.Equal(t, int64(4), tree.Children[2].Comment.ID)
	require.Equal(t, int64(5), tree.Children[1].Children[0].Comment.ID)
	require.Equal(t, int64(6), tree.Children[1].Children[1].Comment.ID)
}
