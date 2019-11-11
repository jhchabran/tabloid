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
	err := s.db.Select(&stories, "SELECT * FROM stories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}

	return stories, nil
}

func (s *PGStore) FindStory(ID string) (*tabloid.Story, error) {
	story := tabloid.Story{}
	err := s.db.Get(&story, "SELECT * FROM stories WHERE id=$1", ID)
	if err != nil {
		return nil, err
	}

	return &story, nil
}

func (s *PGStore) InsertStory(story *tabloid.Story) error {
	var id int64
	err := s.db.Get(&id, "INSERT INTO stories (title, url, body, score, author, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		story.Title, story.URL, story.Body, story.Score, story.Author, time.Now(),
	)

	if err != nil {
		return err
	}

	story.ID = id

	return nil
}

func (s *PGStore) ListComments(storyID string) ([]*tabloid.Comment, error) {
	comments := []*tabloid.Comment{}
	err := s.db.Select(&comments, "SELECT * FROM comments WHERE parent_id=$1 ORDER BY created_at DESC", storyID)
	if err != nil {
		return nil, err
	}

	return comments, nil
}

func (s *PGStore) InsertComment(comment *tabloid.Comment) error {
	var id int64
	err := s.db.Get(&id, "INSERT INTO comments (parent_id, body, score, author, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		comment.ParentID, comment.Body, comment.Score, comment.Author, time.Now(),
	)

	if err != nil {
		return err
	}

	comment.ID = id

	return nil
}
