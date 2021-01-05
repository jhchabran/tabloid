package tabloid

type Store interface {
	Connect() error
	FindStory(ID string) (*Story, error)
	ListStories(page int, perPage int) ([]*Story, error)
	ListStoriesWithVotes(userID string, page int, perPage int) ([]*StorySeenByUser, error)
	InsertStory(item *Story) error
	FindComment(commentID string) (*Comment, error)
	ListComments(storyID string) ([]*Comment, error)
	ListCommentsWithVotes(storyID string, userID string) ([]*CommentSeenByUser, error)
	InsertComment(comment *Comment) error
	UpdateComment(comment *Comment) error
	FindUserByLogin(login string) (*User, error)
	CreateOrUpdateUser(login string, email string) (string, error)
	CreateOrUpdateVoteOnStory(storyID string, userID string, up bool) error
	CreateOrUpdateVoteOnComment(storyID string, userID string, up bool) error
	UpdateUser(user *User) error
}
