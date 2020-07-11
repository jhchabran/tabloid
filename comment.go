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
	AuthorID        int64         `db:"author_id"`
	Author          string        `db:"author"`
	CreatedAt       time.Time     `db:"created_at"`
}

// TODO move this in a better place
type CommentPresenter struct {
	ID         int64
	StoryID    int64
	Path       string
	ParentPath string
	StoryPath  string
	Body       string
	Score      int64
	Author     string
	CreatedAt  time.Time
	Children   []*CommentPresenter
}

// CommentTree is a simple tree of comments ordered by score
type CommentNode struct {
	Comment  *Comment
	Children []*CommentNode
}

// TODO ordering?
func NewCommentPresentersTree(comments []*Comment) []*CommentPresenter {
	index := map[sql.NullInt64]*CommentNode{}
	var roots []*CommentNode

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
			roots = append(roots, node)
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

	// Turn the nodes into presenters
	result := []*CommentPresenter{}
	for _, n := range roots {
		result = append(result, NewCommentPresenter(n))
	}
	return result
}

func NewComment(storyID int64, parentCommentID sql.NullInt64, body string, authorID int64) *Comment {
	return &Comment{
		ParentCommentID: parentCommentID,
		StoryID:         storyID,
		Body:            body,
		AuthorID:        authorID,
		CreatedAt:       time.Now(),
		Upvotes:         1,
	}
}

func (c *Comment) Score() int64 {
	return c.Upvotes - c.Downvotes
}

// TODO missing fields
func NewCommentPresenter(c *CommentNode) *CommentPresenter {
	var children []*CommentPresenter

	for _, c := range c.Children {
		children = append(children, NewCommentPresenter(c))
	}

	return &CommentPresenter{
		ID:        c.Comment.ID,
		StoryID:   c.Comment.StoryID,
		Body:      c.Comment.Body,
		Score:     c.Comment.Score(),
		Author:    c.Comment.Author,
		CreatedAt: c.Comment.CreatedAt,
		Children:  children,
	}
}
