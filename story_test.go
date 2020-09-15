package tabloid

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestOK(t *testing.T) {
	c := qt.New(t)

	var story *Story
	var userID int64 = 1
	now, _ := time.Parse(time.RFC3339, "2020-01-01T12:00:00Z")
	nowF := func() time.Time { return now }

	withFakeNow(nowF, func() {
		story = NewStory("foo", "body", userID, "http://foobar.com")
		c.Assert(now, qt.Equals, story.CreatedAt)
	})
}

func withFakeNow(nowFunc func() time.Time, f func()) {
	old := NowFunc
	NowFunc = nowFunc
	defer func() { NowFunc = old }()
	f()
}
