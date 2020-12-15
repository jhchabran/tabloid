package tabloid

import (
	"database/sql"
	"html/template"
	"regexp"
	"time"
)

// A username starts with a @, includes only alphanumerical chars or a hyphen.
// It is preceded by a space or line beginning and is followed by a space or the end of the line.
var usernameRegexp = regexp.MustCompile(`(?:^|\s+)@([[:alnum:]][[:alnum:]-]*[[:alnum:]])(?:$|\s+|[,;.])`)

type Comment struct {
	ID              string         `db:"id"`
	ParentCommentID sql.NullString `db:"parent_comment_id"`
	StoryID         string         `db:"story_id"`
	Body            string         `db:"body"`
	Score           int64          `db:"score"`
	AuthorID        string         `db:"author_id"`
	Author          string         `db:"author"`
	CreatedAt       time.Time      `db:"created_at"`
}

func (c *Comment) GetID() string                      { return c.ID }
func (c *Comment) GetParentCommentID() sql.NullString { return c.ParentCommentID }
func (c *Comment) Pings() []string {
	matches := usernameRegexp.FindAllStringSubmatch(c.Body, -1)

	var res []string
	for _, m := range matches {
		res = append(res, m[1:]...)
	}

	return res
}

type CommentSeenByUser struct {
	Comment
	UserID string       `db:"user_id"`
	Up     sql.NullBool `db:"up"`
}

func (c *CommentSeenByUser) GetID() string                      { return c.ID }
func (c *CommentSeenByUser) GetParentCommentID() sql.NullString { return c.ParentCommentID }

// TODO we should probably refactor these structs, their name are unclear
// for now, it'll do the job.
type CommentAccessor interface {
	GetID() string
	GetParentCommentID() sql.NullString
}

// TODO move this in a better place
type CommentPresenter struct {
	ID         string
	StoryID    string
	Path       string
	ParentPath string
	StoryPath  string
	Body       template.HTML
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
	index := map[sql.NullString]*CommentNode{}
	var roots []*CommentNode

	for _, c := range comments {
		// create the node
		var node *CommentNode
		id := sql.NullString{String: c.GetID(), Valid: true}
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

func NewComment(storyID string, parentCommentID sql.NullString, body string, authorID string) *Comment {
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
			Body:      renderBody(comment.Body),
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
			Body:      renderBody(comment.Body),
			Score:     comment.Score,
			Author:    comment.Author,
			CreatedAt: comment.CreatedAt,
			Children:  children,
		}
	}
}
