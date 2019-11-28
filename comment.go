package tabloid

import (
	"database/sql"
	"time"
)

type Comment struct {
	ID              int64         `db:"id"`
	ParentCommentID sql.NullInt64 `db:"parent_comment_id"`
	StoryID         int64         `db:"story_id"`
	Body            string        `db:"body"`
	Upvotes         int64         `db:"upvotes"`
	Downvotes       int64         `db:"downvotes"`
	Author          string        `db:"author"`
	CreatedAt       time.Time     `db:"created_at"`
}

// TODO move this in a better place
type CommentPresenter struct {
	Path       string
	ParentPath string
	StoryPath  string
	Body       string
	Score      int64
	Author     string
	CreatedAt  time.Time
}

// CommentTree is a simple tree of comments ordered by score
type CommentNode struct {
	Comment  *Comment
	Children []*CommentNode
}

// TODO ordering?
func NewCommentTree(comments []*Comment) *CommentNode {
	index := map[sql.NullInt64]*CommentNode{}
	var root *CommentNode

	for _, c := range comments {
		// create the node
		var node *CommentNode
		id := sql.NullInt64{Int64: c.ID, Valid: true}
		if _, ok := index[id]; !ok {
			index[id] = &CommentNode{
				Children: []*CommentNode{},
			}
		}

		node = index[id]
		node.Comment = c

		if !c.ParentCommentID.Valid {
			root = node
		}

		// create parent if it doesn't exists yet
		if _, ok := index[c.ParentCommentID]; !ok {
			index[c.ParentCommentID] = &CommentNode{
				Comment:  nil,
				Children: []*CommentNode{},
			}
		}

		parent := index[c.ParentCommentID]

		// assign child to parent
		parent.Children = append(parent.Children, node)
	}

	return root
}

// parent cannot be nil or it'll crash, but this should never happen
func add(c *Comment, parent *CommentNode) *CommentNode {
	node := &CommentNode{
		Comment:  c,
		Children: []*CommentNode{},
	}

	parent.Children = append(parent.Children, node)

	return node
}

func NewComment(storyID int64, parentCommentID sql.NullInt64, body string, author string) *Comment {
	return &Comment{
		ParentCommentID: parentCommentID,
		StoryID:         storyID,
		Body:            body,
		Author:          author,
		CreatedAt:       time.Now(),
	}
}

func (c *Comment) Score() int64 {
	return c.Upvotes - c.Downvotes
}

// TODO missing fields
func NewCommentPresenter(c *Comment) *CommentPresenter {
	return &CommentPresenter{
		Body:   c.Body,
		Score:  c.Score(),
		Author: c.Author,
	}
}
