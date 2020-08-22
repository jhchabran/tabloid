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
	Score           int64         `db:"score"`
	AuthorID        int64         `db:"author_id"`
	Author          string        `db:"author"`
	CreatedAt       time.Time     `db:"created_at"`
}

func (c *Comment) GetID() int64                      { return c.ID }
func (c *Comment) GetParentCommentID() sql.NullInt64 { return c.ParentCommentID }

type CommentSeenByUser struct {
	Comment
	UserId int64        `db:"user_id"`
	Up     sql.NullBool `db:"up"`
}

func (c *CommentSeenByUser) GetID() int64                      { return c.ID }
func (c *CommentSeenByUser) GetParentCommentID() sql.NullInt64 { return c.ParentCommentID }

// TODO we should probably refactor these structs, their name are unclear
// for now, it'll do the job.
type CommentAccessor interface {
	GetID() int64
	GetParentCommentID() sql.NullInt64
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
	Upvoted    bool
}

// CommentTree is a simple tree of comments ordered by score
type CommentNode struct {
	Comment  CommentAccessor
	Children []*CommentNode
}

// TODO ordering?
func NewCommentPresentersTree(comments []CommentAccessor) []*CommentPresenter {
	index := map[sql.NullInt64]*CommentNode{}
	var roots []*CommentNode

	for _, c := range comments {
		// create the node
		var node *CommentNode
		id := sql.NullInt64{Int64: c.GetID(), Valid: true}
		if _, ok := index[id]; !ok {
			index[id] = &CommentNode{
				Children: []*CommentNode{},
			}
		}

		node = index[id]
		node.Comment = c

		if !c.GetParentCommentID().Valid {
			roots = append(roots, node)
		}

		// create parent if it doesn't exists yet
		if _, ok := index[c.GetParentCommentID()]; !ok {
			index[c.GetParentCommentID()] = &CommentNode{
				Comment:  nil,
				Children: []*CommentNode{},
			}
		}

		parent := index[c.GetParentCommentID()]

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
	}
}

// TODO missing fields
func NewCommentPresenter(c *CommentNode) *CommentPresenter {
	var children []*CommentPresenter

	for _, c := range c.Children {
		children = append(children, NewCommentPresenter(c))
	}

	if comment, ok := c.Comment.(*CommentSeenByUser); ok {
		return &CommentPresenter{
			ID:        comment.ID,
			StoryID:   comment.StoryID,
			Body:      comment.Body,
			Score:     comment.Score,
			Author:    comment.Author,
			CreatedAt: comment.CreatedAt,
			Children:  children,
			Upvoted:   comment.Up.Bool,
		}
	} else {
		comment, _ := c.Comment.(*Comment)
		return &CommentPresenter{
			ID:        comment.ID,
			StoryID:   comment.StoryID,
			Body:      comment.Body,
			Score:     comment.Score,
			Author:    comment.Author,
			CreatedAt: comment.CreatedAt,
			Children:  children,
		}
	}
}
