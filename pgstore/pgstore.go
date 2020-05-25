package pgstore

import (
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type PGStore struct {
	dbString string
	db       *sqlx.DB
}

func New(addr string) *PGStore {
	return &PGStore{
		dbString: addr,
	}
}

func (s *PGStore) Connect() error {
	db, err := sqlx.Connect("postgres", s.dbString)
	if err != nil {
		return err
	}

	s.db = db

	return nil
}

func (s *PGStore) DB() *sqlx.DB {
	return s.db
}

func (s *PGStore) ListStories() ([]*tabloid.Story, error) {
	stories := []*tabloid.Story{}
	err := s.db.Select(&stories, "SELECT stories.*, users.name as author FROM stories JOIN users ON stories.author_id = users.id ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}

	return stories, nil
}

func (s *PGStore) FindStory(ID string) (*tabloid.Story, error) {
	story := tabloid.Story{}
	err := s.db.Get(&story, "SELECT stories.*, users.name as author FROM stories JOIN users ON stories.author_id = users.id WHERE stories.id=$1", ID)
	if err != nil {
		return nil, err
	}

	return &story, nil
}

func (s *PGStore) InsertStory(story *tabloid.Story) error {
	var id int64
	err := s.db.Get(&id, "INSERT INTO stories (title, url, body, score, author_id, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		story.Title, story.URL, story.Body, story.Score, story.AuthorID, time.Now(),
	)

	if err != nil {
		return err
	}

	story.ID = id

	return nil
}

func (s *PGStore) ListComments(storyID string) ([]*tabloid.Comment, error) {
	comments := []*tabloid.Comment{}
	err := s.db.Select(&comments, "SELECT comments.*, users.name as author FROM comments JOIN users ON comments.author_id = users.id WHERE story_id=$1 ORDER BY comments.created_at DESC", storyID)
	if err != nil {
		return nil, err
	}

	return comments, nil
}

func (s *PGStore) InsertComment(comment *tabloid.Comment) error {
	var id int64
	err := s.db.Get(&id, "INSERT INTO comments (story_id, parent_comment_id, body, upvotes, downvotes, author_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		comment.StoryID, comment.ParentCommentID, comment.Body, comment.Upvotes, comment.Downvotes, comment.AuthorID, time.Now(),
	)

	if err != nil {
		return err
	}

	comment.ID = id

	return nil
}

func (s *PGStore) FindUserByLogin(name string) (*tabloid.User, error) {
	user := tabloid.User{}
	err := s.db.Get(&user, "SELECT * FROM users WHERE name=$1", name)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *PGStore) CreateOrUpdateUser(login string, email string) error {
	now := time.Now()
	_, err := s.db.Exec("INSERT INTO users (name, email, created_at, last_login_at) VALUES ($1, $2, $3, $4) ON CONFlICT (name) DO UPDATE SET last_login_at = $5", login, email, now, now, now)

	if err != nil {
		return err
	}

	return nil
}
