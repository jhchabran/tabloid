package tabloid

type Store interface {
	Connect() error
	FindStory(ID string) (*Story, error)
	ListStories(page int, perPage int) ([]*Story, error)
	ListStoriesWithVotes(userID int64, page int, perPage int) ([]*StorySeenByUser, error)
	InsertStory(item *Story) error
	FindComment(commentID string) (*Comment, error)
	ListComments(storyID string) ([]*Comment, error)
	ListCommentsWithVotes(storyID string, userID int64) ([]*CommentSeenByUser, error)
	InsertComment(comment *Comment) error
	FindUserByLogin(login string) (*User, error)
	CreateOrUpdateUser(login string, email string) error
	CreateOrUpdateVoteOnStory(storyID int64, userID int64, up bool) error
	CreateOrUpdateVoteOnComment(storyID int64, userID int64, up bool) error
}
