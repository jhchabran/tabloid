package tabloid

type Store interface {
	Connect() error
	FindStory(ID string) (*Story, error)
	ListStories() ([]*Story, error)
	InsertStory(item *Story) error
	ListComments(storyID string) ([]*Comment, error)
	InsertComment(comment *Comment) error
}
