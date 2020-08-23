package tabloid

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCommentOK(t *testing.T) {
	comment := NewComment(1, sql.NullInt64{Int64: 1, Valid: true}, "body", 1)
	require.Equal(t, int64(1), comment.ParentCommentID.Int64)
	require.True(t, comment.ParentCommentID.Valid)
}

func TestNewCommentPresentersTree(t *testing.T) {
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

	require.Equal(t, int64(1), ps[0].ID)
	require.Equal(t, int64(2), ps[0].Children[0].ID)
	require.Equal(t, int64(3), ps[0].Children[1].ID)
	require.Equal(t, int64(4), ps[0].Children[2].ID)
	require.Equal(t, int64(5), ps[0].Children[1].Children[0].ID)
	require.Equal(t, int64(6), ps[0].Children[1].Children[1].ID)
	require.Equal(t, int64(7), ps[1].ID)
	require.Equal(t, int64(8), ps[1].Children[0].ID)
}
