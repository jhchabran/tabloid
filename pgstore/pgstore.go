package pgstore

import (
	"time"

	"github.com/jhchabran/tabloid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// A PGStore is responsible of interacting with the storage layer using a Postgresql database.
type PGStore struct {
	dbString string
	db       *sqlx.DB
}

// New returns a PGStore configured for a given address string, using the "user=postgres dbname=tabloid ..." format.
func New(addr string) *PGStore {
	return &PGStore{
		dbString: addr,
	}
}

// Connect establish a connection with the database using the address given at initialization.
func (s *PGStore) Connect() error {
	db, err := sqlx.Connect("postgres", s.dbString)
	if err != nil {
		return err
	}

	s.db = db

	return nil
}

// DB returns the existing connection, making it suitable to perform requests not already supported by
// the store interface. If called while not connected, it will return nil.
func (s *PGStore) DB() *sqlx.DB {
	return s.db
}

// https://www.citusdata.com/blog/2016/03/30/five-ways-to-paginate/
func (s *PGStore) ListStories(page int, perPage int) ([]*tabloid.Story, error) {
	stories := []*tabloid.Story{}
	err := s.db.Select(&stories, "SELECT stories.*, users.name as author FROM stories JOIN users ON stories.author_id = users.id ORDER BY created_at DESC LIMIT $1 OFFSET $2", perPage, page*perPage)
	if err != nil {
		return nil, err
	}

	return stories, nil
}

func (s *PGStore) ListStoriesWithVotes(userID int64, page int, perPage int) ([]*tabloid.StorySeenByUser, error) {
	stories := []*tabloid.StorySeenByUser{}
	err := s.db.Select(&stories,
		`SELECT stories.*, users.name as author, users.id as user_id, votes.up as up
		FROM stories
		JOIN users ON stories.author_id = users.id
		LEFT JOIN votes ON stories.id = votes.story_id AND votes.user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, perPage, page*perPage)
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
	now := time.Now()
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = sqlx.Get(
		tx,
		&id,
		"INSERT INTO stories (title, url, body, author_id, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		story.Title, story.URL, story.Body, story.AuthorID, now,
	)

	if err != nil {
		return err
	}

	story.ID = id

	// a story being created_at always comes with its accompanying upvote from its submitter.
	_, err = tx.Exec(
		"INSERT INTO votes (story_id, up, user_id, created_at) VALUES ($1, $2, $3, $4)",
		id, true, story.AuthorID, now)

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// we don't need to re-read from the database, it's the story creation, it can only have one upvote, the one
	// from the author, added by a trigger on the upvote table
	story.Score = 1

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

func (s *PGStore) CreateOrUpdateVoteOnStory(storyID int64, userID int64, up bool) error {
	now := time.Now()
	_, err := s.db.Exec("INSERT INTO votes (story_id, user_id, up, created_at) VALUES ($1, $2, $3, $4) ON CONFlICT (user_id, story_id) WHERE comment_id IS NULL DO UPDATE SET up = $5",
		storyID, userID, up, now, up)

	if err != nil {
		return err
	}

	return nil
}
