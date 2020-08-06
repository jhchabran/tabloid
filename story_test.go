package tabloid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOK(t *testing.T) {
	r := require.New(t)

	var story *Story
	var userID int64 = 1
	now, _ := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	nowF := func() time.Time { return now }

	withFakeNow(nowF, func() {
		story = NewStory("foo", "body", userID, "http://foobar.com")
		r.Equal(now, story.CreatedAt)
	})
}

func withFakeNow(nowFunc func() time.Time, f func()) {
	old := NowFunc
	NowFunc = nowFunc
	defer func() { NowFunc = old }()
	f()
}
