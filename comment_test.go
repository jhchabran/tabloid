package tabloid

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jhchabran/tabloid/ranking"
)

func TestNewCommentOK(t *testing.T) {
	c := qt.New(t)

	comment := NewComment("1", sql.NullString{String: "1", Valid: true}, "body", "1")
	c.Assert("1", qt.Equals, comment.ParentCommentID.String)
	c.Assert(comment.ParentCommentID.Valid, qt.IsTrue)
}

func TestNewCommentPresentersTree(t *testing.T) {
	c := qt.New(t)

	authorID := "42"
	nilParent := sql.NullString{String: "", Valid: false}
	a := NewComment("1", nilParent, "body", authorID)
	a.ID = "1"
	a1 := NewComment("1", sql.NullString{String: a.ID, Valid: true}, "", authorID)
	a1.ID = "2"
	a2 := NewComment("1", sql.NullString{String: a.ID, Valid: true}, "", authorID)
	a2.ID = "3"
	a3 := NewComment("1", sql.NullString{String: a.ID, Valid: true}, "", authorID)
	a3.ID = "4"
	a21 := NewComment("1", sql.NullString{String: a2.ID, Valid: true}, "", authorID)
	a21.ID = "5"
	a22 := NewComment("1", sql.NullString{String: a2.ID, Valid: true}, "", authorID)
	a22.ID = "6"
	b := NewComment("1", nilParent, "body", authorID)
	b.ID = "7"
	b1 := NewComment("1", sql.NullString{String: b.ID, Valid: true}, "", authorID)
	b1.ID = "8"

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

	tests := map[string]string{
		"1": ps[0].ID,
		"2": ps[0].Children[0].ID,
		"3": ps[0].Children[1].ID,
		"4": ps[0].Children[2].ID,
		"5": ps[0].Children[1].Children[0].ID,
		"6": ps[0].Children[1].Children[1].ID,
		"7": ps[1].ID,
		"8": ps[1].Children[0].ID,
	}

	for want, input := range tests {
		c.Assert(input, qt.Equals, want)
	}
}

func TestSortCommentsByRank(t *testing.T) {
	c := qt.New(t)

	comments := []*Comment{}
	nextID := 1

	nc := func(parentID string, score int) string {
		var c *Comment
		if parentID != "" {
			c = NewComment("1", sql.NullString{String: parentID, Valid: true}, "", "42")
		} else {
			c = NewComment("1", sql.NullString{}, "", "42")
		}

		c.ID = strconv.Itoa(nextID)
		c.Score = int64(score)
		nextID++
		comments = append(comments, c)

		return c.ID
	}

	nc("", 10)
	nc("", 20)
	id := nc("", 30) // id = 3
	nc(id, 1)
	nc(id, 2)
	nc(id, 3)
	nc(id, 4)
	nc(id, 5)
	id = nc("", 4) // id = 9
	nc(id, 1)
	nc(id, 2)
	nc(id, 3)

	cc := make([]CommentAccessor, len(comments))
	for i, c := range comments {
		cc[i] = c
	}

	tree := NewCommentPresentersTree(cc)
	tree.Sort(func(r ranking.Rankable) float64 {
		return ranking.Rank(r, 1.8, 1, NowFunc())
	})

	c.Assert(tree[0].Score, qt.Equals, int64(30)) // id = 3
	c.Assert(tree[1].Score, qt.Equals, int64(20))
	c.Assert(tree[2].Score, qt.Equals, int64(10))

	c.Assert(tree[0].Children[0].Score, qt.Equals, int64(5))
	c.Assert(tree[0].Children[1].Score, qt.Equals, int64(4))
}

func TestPings(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		body string
		want []string
	}{
		{"@foo may be interested", []string{"foo"}},
		{"@foo and @bar may be interested", []string{"foo", "bar"}},
		{"hey @foo-a and @foo-b", []string{"foo-a", "foo-b"}},
		{"hey @foo, @bar", []string{"foo", "bar"}},
		{"ping @foo. @bar knows", []string{"foo", "bar"}},
		{"contact foo@bar.com", nil},
		{"try user@localhost it should work", nil},
	}

	for _, t := range tests {
		c.Run(fmt.Sprintf("'%s' must return (%v)", t.body, t.want), func(c *qt.C) {
			comment := NewComment("1", sql.NullString{}, t.body, "1")
			c.Assert(comment.Pings(), qt.DeepEquals, t.want)
		})
	}

}
