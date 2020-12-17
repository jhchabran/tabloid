package integration

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	qt "github.com/frankban/quicktest"
	"github.com/jhchabran/tabloid"
)

func TestIndexPage(t *testing.T) {
	c := qt.New(t)

	c.Run("OK unauthenticated empty index page", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		resp, err := http.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(200, qt.Equals, resp.StatusCode)
	})

	c.Run("OK unauthenticated single story index page", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		id, err := tc.createUser("alpha")
		c.Assert(err, qt.IsNil)

		err = tc.pgStore.InsertStory(&tabloid.Story{
			Title:     "Foobar",
			URL:       "http://foobar.com",
			Body:      "Foobaring",
			AuthorID:  id,
			CreatedAt: time.Now(),
		})
		c.Assert(err, qt.IsNil)

		resp, err := http.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(200, qt.Equals, resp.StatusCode)
		defer resp.Body.Close()
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		c.Assert("Tabloid", qt.Equals, doc.Find("title").Text())
		a := doc.Find("a.story-url")
		url := a.AttrOr("href", "")
		text := a.Text()
		c.Assert(url, qt.Equals, "http://foobar.com")
		c.Assert(text, qt.Equals, "Foobar")
	})

	// 7 items, 3 per page
	c.Run("OK pagination", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		id, err := tc.createUser("alpha")
		c.Assert(err, qt.IsNil)

		for i := 0; i < 7; i++ {
			ii := strconv.Itoa(i)
			err := tc.pgStore.InsertStory(&tabloid.Story{
				Title:     "Foobar" + ii,
				URL:       "http://foobar.com/" + ii,
				Body:      "Foobaring",
				AuthorID:  id,
				CreatedAt: tabloid.NowFunc(),
			})
			c.Assert(err, qt.IsNil)
		}

		client := tc.newAuthenticatedClient()

		// newTestContext initializes the perPage count to 3
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		c.Run("results are paginated", func(c *qt.C) {
			count := doc.Find(".story-item").Length()
			c.Assert(count, qt.Equals, 3)
		})

		c.Run("have a link to the next page on the home", func(c *qt.C) {
			link := doc.Find("a.pagination")
			_, ok := link.Attr("href")

			c.Assert(ok, qt.IsTrue)
			c.Assert(link.Length(), qt.Equals, 1)
			c.Assert(link.Text(), qt.Contains, "Next")
		})

		c.Run("have a prev and next link on the second page", func(c *qt.C) {
			link := doc.Find("a.pagination")
			href, ok := link.Attr("href")
			c.Assert(ok, qt.IsTrue)

			// go to the second page
			resp, err := client.Get(tc.url(href))
			c.Assert(err, qt.IsNil)
			defer resp.Body.Close()

			// read the dom
			ddoc, err := goquery.NewDocumentFromReader(resp.Body)
			c.Assert(err, qt.IsNil)

			// there should be two pagination links
			links := ddoc.Find("a.pagination")
			c.Assert(links.Length(), qt.Equals, 2)
			c.Assert(links.Text(), qt.Contains, "Prev")
			c.Assert(links.Text(), qt.Contains, "Next")
		})

		c.Run("prev link sends back to the the previous page", func(c *qt.C) {
			link := doc.Find("a.pagination")
			href, ok := link.Attr("href")
			c.Assert(ok, qt.IsTrue)

			// go to the second page
			resp, err := client.Get(tc.url(href))
			c.Assert(err, qt.IsNil)
			defer resp.Body.Close()

			// read the dom
			ddoc, err := goquery.NewDocumentFromReader(resp.Body)
			c.Assert(err, qt.IsNil)

			// there should be two pagination links and we want the first one, Prev
			link = ddoc.Find("a.pagination").First()
			c.Assert(link.Text(), qt.Equals, "Prev")
			c.Assert(link.AttrOr("href", ""), qt.Equals, "/?page=0")
		})

		c.Run("no next link on the last page", func(c *qt.C) {
			// go to the second page
			link := doc.Find("a.pagination")
			href, ok := link.Attr("href")
			c.Assert(ok, qt.IsTrue)

			resp, err := client.Get(tc.url(href))
			c.Assert(err, qt.IsNil)
			defer resp.Body.Close()

			// read the dom
			ddoc, err := goquery.NewDocumentFromReader(resp.Body)
			c.Assert(err, qt.IsNil)

			// go to the third page
			link = ddoc.Find("a.pagination").Last()
			href, ok = link.Attr("href")
			c.Assert(ok, qt.IsTrue)

			resp, err = client.Get(tc.url(href))
			c.Assert(err, qt.IsNil)
			defer resp.Body.Close()

			// read the dom
			ddoc, err = goquery.NewDocumentFromReader(resp.Body)
			c.Assert(err, qt.IsNil)

			// there should be two pagination links and we want the first one, Prev
			link = ddoc.Find("a.pagination").First()
			c.Assert(link.Text(), qt.Equals, "Prev")
			c.Assert(link.AttrOr("href", ""), qt.Equals, "/?page=1")
			count := ddoc.Find("a.pagination").Length()
			c.Assert(count, qt.Equals, 1)
		})
	})
}

func TestSubmitStory(t *testing.T) {
	c := qt.New(t)

	c.Run("there is no submit link when not authenticated", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newHTTPClient()
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		submitEl := doc.Find("nav a").FilterFunction(func(_ int, sel *goquery.Selection) bool {
			return sel.Text() == "Submit"
		}).Length()
		c.Assert(submitEl, qt.Equals, 0)
	})

	c.Run("cannot submit a story while not authenticated", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newHTTPClient()
		values := url.Values{
			"title": []string{"Captain Nemo"},
			"url":   []string{"http://duckduckgo.com"},
			"body":  []string{"foobar"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 401)
	})

	c.Run("cannot submit with a link and a really long title", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		b := make([]byte, 64+1)
		for i := 0; i < 64+1; i++ {
			b[i] = 'a'
		}

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{string(b)},
			"url":   []string{"http://duckduckgo.com"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 400)
	})

	c.Run("submit with a link and a title", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Captain Nemo"},
			"url":   []string{"http://duckduckgo.com"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)
	})

	c.Run("submit with a link, a body and a title", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Captain Nemo"},
			"url":   []string{"http://duckduckgo.com"},
			"body":  []string{"Here is a great link"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)
	})

	c.Run("submit without a link,but with a body and a title", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"How do I git gud at coding"},
			"body":  []string{"Someone told me I must learn assembly"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)
	})

	c.Run("cannot submit without a link but with a title", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Captain Nemo"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 400)
	})

	c.Run("cannot submit without a link and without a title but with a body", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"body": []string{"errrrr"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 400)
	})

	c.Run("trim spaces on title when submitting a story", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Foo      "},
			"url":   []string{"http://foobar"},
		}

		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		title := doc.Find("a.story-title").Text()
		c.Assert(title, qt.Equals, "Foo")
	})

	c.Run("trim spaces on url when submitting a story", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Foo"},
			"url":   []string{"http://foobar.com/         "},
		}

		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		href := doc.Find("a.story-title").AttrOr("href", "")
		c.Assert(href, qt.Equals, "http://foobar.com/")
	})

	c.Run("trim spaces on body when submitting a story", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Foo"},
			"url":   []string{"http://foobar.com/"},
			"body":  []string{"space\nnow      \n\n     "},
		}

		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		body := doc.Find(".story-body").Text()
		c.Assert(strings.TrimSpace(body), qt.Equals, "space\nnow")
	})

	c.Run("reject an invalid url when submitting a story", func(c *qt.C) {
		tests := []struct {
			url    string
			status int
		}{
			{"httpfoobar.com/", 400},
			{"httpfoobar", 400},
			{"ftp://foobar-com/", 400},
			{"http://foobar.com/", 200},
			{"https://foobar.com/", 200},
		}

		tc := newTestContext(c)
		tc.prepareServer()
		client := tc.newAuthenticatedClient()

		for _, test := range tests {
			values := url.Values{
				"title": []string{"Foo"},
				"url":   []string{test.url},
			}

			resp, err := client.PostForm(tc.url("/submit"), values)
			c.Assert(err, qt.IsNil, qt.Commentf("url=%v", test.url))
			defer resp.Body.Close()
			c.Assert(resp.StatusCode, qt.Equals, test.status, qt.Commentf("url=%v", test.url))
		}
	})

}

func TestAuthentication(t *testing.T) {
	c := qt.New(t)

	c.Run("signing in", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()
		client := tc.newHTTPClient()

		resp, err := client.Get(tc.url("/oauth/start"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		login := doc.Find("a#session-login").Text()
		c.Assert(login, qt.Contains, "fakeLogin")
	})

	c.Run("signing out", func(c *qt.C) {
		tc := newTestContext(c)
		tc.prepareServer()
		client := tc.newAuthenticatedClient()

		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		logoutPath, ok := doc.Find("a#session-signout").Attr("href")
		c.Assert(ok, qt.IsTrue)

		resp, err = client.Get(tc.url(logoutPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		_, ok = doc.Find("a#session-signin").Attr("href")
		c.Assert(ok, qt.IsTrue)
	})
}

func TestStoryVoting(t *testing.T) {
	c := qt.New(t)
	tc := newTestContext(c)
	tc.prepareServer()

	id, err := tc.createUser("alpha")
	c.Assert(err, qt.IsNil)

	err = tc.pgStore.InsertStory(&tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Body:      "Foobaring",
		AuthorID:  id,
		CreatedAt: tabloid.NowFunc(),
	})
	c.Assert(err, qt.IsNil)

	client := tc.newAuthenticatedClient()
	resp, err := client.Get(tc.url("/"))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	c.Run("click on the upvote arrow", func(c *qt.C) {
		// Find the upvote button
		action, ok := doc.Find(".voters form.upvoter").Attr("action")
		c.Assert(ok, qt.IsTrue)
		c.Assert(action, qt.Not(qt.IsNil))

		resp, err = client.PostForm(tc.url(action), nil)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// The story score should be 2 (original upvote plus this one)
		c.Assert(doc.Find("span.story-meta").Text(), qt.Contains, "2 by alpha, today")
	})

	c.Run("upvote button should disappear after voting", func(c *qt.C) {
		_, ok := doc.Find(".voters form.upvoter button").Attr("disabled")
		c.Assert(ok, qt.IsTrue, qt.Commentf("disabled attribute must be present on the button"))
	})

	c.Run("click on the upvet arrow when unauthenticated should redirect to login", func(c *qt.C) {
		client := tc.newHTTPClient()
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		href, ok := doc.Find("a.voters-inactive").Attr("href")
		c.Assert(ok, qt.IsTrue, qt.Commentf("cannot find placeholder for unathenticated upvotes"))
		c.Assert(href, qt.Equals, "/oauth/start")
	})

	c.Run("click on the upvote arrow with a different user", func(c *qt.C) {
		// Login with a different user, the fake_auth package will create a new one for each subsequent login
		client := tc.newAuthenticatedClient()
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// Find the upvote button
		action, ok := doc.Find(".voters form.upvoter").Attr("action")
		c.Assert(ok, qt.IsTrue)
		c.Assert(action, qt.Not(qt.IsNil))

		resp, err = client.PostForm(tc.url(action), nil)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// The story score should now be 3
		c.Assert(doc.Find("span.story-meta").Text(), qt.Contains, "3 by alpha, today")
	})
}

func TestCommentsSubmiting(t *testing.T) {
	c := qt.New(t)
	tc := newTestContext(c)
	tc.prepareServer()

	id, err := tc.createUser("alpha")
	c.Assert(err, qt.IsNil)

	// create a story to comment on
	story := &tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Body:      "Foobaring",
		AuthorID:  id,
		CreatedAt: tabloid.NowFunc(),
	}
	err = tc.pgStore.InsertStory(story)
	c.Assert(err, qt.IsNil)

	client := tc.newAuthenticatedClient()
	resp, err := client.Get(tc.url("/"))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	storyPath, ok := doc.Find("a.story-comments").Attr("href")
	c.Assert(ok, qt.IsTrue)

	c.Run("story comments count is pluralized when there are zero comments", func(c *qt.C) {
		c.Assert(doc.Find("a.story-comments").Text(), qt.Contains, "0 Comments")
	})

	c.Run("submit button is disabled for unauthenticated users", func(c *qt.C) {
		client := tc.newHTTPClient()

		resp, err := client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		_, ok := doc.Find("form.new-comment-form input[type=submit]").Attr("disabled")
		c.Assert(ok, qt.IsTrue)
	})

	c.Run("cannot post a comment while unauthenticated", func(c *qt.C) {
		client := tc.newHTTPClient()

		resp, err := client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok := doc.Find("form.new-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		values := url.Values{
			"body":      []string{"very insightful comment"},
			"parent-id": []string{""},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 401)
		defer resp.Body.Close()
	})

	c.Run("submit a comment", func(c *qt.C) {
		resp, err := client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok := doc.Find("form.new-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		values := url.Values{
			"body":      []string{"very insightful comment"},
			"parent-id": []string{""},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		c.Assert(doc.Find(".comment-body").Text(), qt.Contains, "very insightful comment")
		c.Assert(doc.Find(".comment-meta").Text(), qt.Contains, "fakeLogin1, 1 point, today")
	})

	c.Run("story comments count is singular when there is one comment", func(c *qt.C) {
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		c.Assert(doc.Find("a.story-comments").Text(), qt.Contains, "1 Comment")
	})

	c.Run("submitting a subcomment", func(c *qt.C) {
		resp, err := client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok := doc.Find("form.new-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		parentCommentID, ok := doc.Find("input[type=hidden][name=parent-id][value!='']").Attr("value")
		c.Assert(ok, qt.IsTrue)
		c.Assert(parentCommentID, qt.Not(qt.Equals), "")

		values := url.Values{
			"body":      []string{"colorful comment"},
			"parent-id": []string{parentCommentID},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		c.Assert(doc.Find(".comment-body").Text(), qt.Contains, "colorful comment")
		c.Assert(doc.Find(".comment-meta").Text(), qt.Contains, "fakeLogin1, 1 point, today")
	})

	c.Run("submitted comments are trimmed from whitespaces", func(c *qt.C) {
		resp, err := client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok := doc.Find("form.new-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		values := url.Values{
			"body":      []string{"very insightful \ncomment           \n                          "},
			"parent-id": []string{""},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		c.Assert(doc.Find(".comment-body").Text(), qt.Contains, "very insightful comment")
	})

}

func TestCommentsEditing(t *testing.T) {
	c := qt.New(t)
	tc := newTestContext(c)
	tc.prepareServer()

	id, err := tc.createUser("alpha")
	c.Assert(err, qt.IsNil)

	// create a story to comment on
	story := &tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Body:      "Foobaring",
		AuthorID:  id,
		CreatedAt: tabloid.NowFunc(),
	}
	err = tc.pgStore.InsertStory(story)
	c.Assert(err, qt.IsNil)

	client := tc.newAuthenticatedClient()
	resp, err := client.Get(tc.url("/"))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	storyPath, ok := doc.Find("a.story-comments").Attr("href")
	c.Assert(ok, qt.IsTrue)

	// go to the story page
	resp, err = client.Get(tc.url(storyPath))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	doc, err = goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	// Post a new comment
	action, ok := doc.Find("form.new-comment-form").First().Attr("action")
	c.Assert(ok, qt.IsTrue)

	values := url.Values{
		"body":      []string{"foo"},
		"parent-id": []string{""},
	}

	resp, err = client.PostForm(tc.url(action), values)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	// find the edit link
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)
	editPath, ok := doc.Find("a.comment-edit").First().Attr("href")
	c.Assert(ok, qt.IsTrue)

	c.Run("OK", func(c *qt.C) {
		// get to that page
		resp, err = client.Get(tc.url(editPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok = doc.Find("form.edit-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		// post the edition form
		values = url.Values{
			"_method": []string{"PUT"},
			"body":    []string{"barbarbar"},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		// we now should be on the story page
		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// check that our comment is now updated
		c.Assert(doc.Find(".comment-body").Text(), qt.Contains, "barbarbar")
	})

	c.Run("link is displayed only on my own comments", func(c *qt.C) {
		// log in with a different user
		otherClient := tc.newAuthenticatedClient()

		// get the story page
		resp, err := otherClient.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		_, ok := doc.Find("a.comment-edit").First().Attr("href")
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("link is not displayed when not authenticated", func(c *qt.C) {
		// log in with a different user
		otherClient := tc.newHTTPClient()

		// get the story page
		resp, err := otherClient.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)
		_, ok := doc.Find("a.comment-edit").First().Attr("href")
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("unauthenticated edit fails", func(c *qt.C) {
		// use an unauthenticated client to edit
		client := tc.newHTTPClient()
		resp, err := client.Get(tc.url(editPath))
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 401)
	})

	c.Run("redirects if trying to edit after edit window", func(c *qt.C) {
		oldNowFunc := tabloid.NowFunc
		tabloid.NowFunc = func() time.Time { return oldNowFunc().Add(100 * time.Hour) } // warp in 100h
		defer func() { tabloid.NowFunc = oldNowFunc }()

		// get to that page
		resp, err = client.Get(tc.url(editPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		_, ok = doc.Find("form.edit-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("edit link isn't shown if comment is out of edit window", func(c *qt.C) {
		oldNowFunc := tabloid.NowFunc
		tabloid.NowFunc = func() time.Time { return oldNowFunc().Add(100 * time.Hour) } // warp in 100h
		defer func() { tabloid.NowFunc = oldNowFunc }()

		// go to the story page
		resp, err = client.Get(tc.url(storyPath))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		_, ok := doc.Find("a.comment-edit").First().Attr("href")
		c.Assert(ok, qt.IsFalse)
	})
}

func TestCommentsVoting(t *testing.T) {
	c := qt.New(t)
	tc := newTestContext(c)
	tc.prepareServer()

	id, err := tc.createUser("alpha")
	c.Assert(err, qt.IsNil)

	// create a story to comment on
	story := &tabloid.Story{
		Title:     "Foobar",
		URL:       "http://foobar.com",
		Body:      "Foobaring",
		AuthorID:  id,
		CreatedAt: tabloid.NowFunc(),
	}
	err = tc.pgStore.InsertStory(story)
	c.Assert(err, qt.IsNil)

	// create a comment to upvote
	comment := tabloid.NewComment(story.ID, sql.NullString{}, "kudos", id)
	err = tc.pgStore.InsertComment(comment)
	c.Assert(err, qt.IsNil)

	client := tc.newAuthenticatedClient()
	resp, err := client.Get(tc.url("/"))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	// navigate to the story page
	path, ok := doc.Find("a.story-comments").Attr("href")
	c.Assert(ok, qt.IsTrue)

	resp, err = client.Get(tc.url(path))
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, 200)
	defer resp.Body.Close()

	doc, err = goquery.NewDocumentFromReader(resp.Body)
	c.Assert(err, qt.IsNil)

	c.Run("click on the upvote arrow", func(c *qt.C) {
		// Find the upvote button
		action, ok := doc.Find(".voters form.upvoter").Attr("action")
		c.Assert(ok, qt.IsTrue)
		c.Assert(action, qt.Not(qt.IsNil))

		resp, err = client.PostForm(tc.url(action), nil)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// The comment score should be 2 (original upvote plus this one)
		c.Assert(doc.Find("span.comment-meta").Text(), qt.Contains, "alpha, 2 points, today")
	})

	c.Run("upvote button should disappear after voting", func(c *qt.C) {
		_, ok := doc.Find(".voters form.upvoter button").Attr("disabled")
		c.Assert(ok, qt.IsTrue, qt.Commentf("disabled attribute must be present on the button"))
	})

	c.Run("click on the upvet arrow when unauthenticated should redirect to login", func(c *qt.C) {
		client := tc.newHTTPClient()
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		href, ok := doc.Find("a.voters-inactive").Attr("href")
		c.Assert(ok, qt.IsTrue, qt.Commentf("cannot find placeholder for unathenticated upvotes"))
		c.Assert(href, qt.Equals, "/oauth/start")
	})

	c.Run("click on the upvote arrow with a different user", func(c *qt.C) {
		// Login with a different user, the fake_auth package will create a new one for each subsequent login
		client := tc.newAuthenticatedClient()
		resp, err := client.Get(tc.url("/"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// find the story link
		path, ok := doc.Find("a.story-comments").Attr("href")
		c.Assert(ok, qt.IsTrue)

		// navigate to the story page
		resp, err = client.Get(tc.url(path))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// Find the upvote button
		action, ok := doc.Find(".voters form.upvoter").Attr("action")
		c.Assert(ok, qt.IsTrue)
		c.Assert(action, qt.Not(qt.IsNil))

		resp, err = client.PostForm(tc.url(action), nil)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()

		doc, err = goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		// The story score should now be 3
		c.Assert(doc.Find("span.comment-meta").Text(), qt.Contains, "alpha, 3 points, today")
	})
}

func TestHooks(t *testing.T) {
	c := qt.New(t)

	c.Run("Submitting a story runs the story hooks", func(c *qt.C) {
		tc := newTestContext(c)
		seen := false
		tc.server.AddStoryHook(func(story *tabloid.Story) error {
			seen = true
			return nil
		})
		tc.prepareServer()

		client := tc.newAuthenticatedClient()
		values := url.Values{
			"title": []string{"Captain Nemo"},
			"url":   []string{"http://duckduckgo.com"},
		}
		resp, err := client.PostForm(tc.url("/submit"), values)
		c.Assert(err, qt.IsNil)
		defer resp.Body.Close()
		c.Assert(resp.StatusCode, qt.Equals, 200)
		c.Assert(seen, qt.IsTrue)
	})

	c.Run("Submitting a comment runs the comment hooks", func(c *qt.C) {
		tc := newTestContext(c)
		seen := false
		tc.server.AddCommentHook(func(story *tabloid.Story, comment *tabloid.Comment) error {
			seen = true
			return nil
		})
		tc.prepareServer()

		id, err := tc.createUser("alpha")
		c.Assert(err, qt.IsNil)

		// create a story to comment on
		story := &tabloid.Story{
			Title:     "Foobar",
			URL:       "http://foobar.com",
			Body:      "Foobaring",
			AuthorID:  id,
			CreatedAt: tabloid.NowFunc(),
		}
		err = tc.pgStore.InsertStory(story)
		c.Assert(err, qt.IsNil)

		client := tc.newAuthenticatedClient()
		resp, err := client.Get(tc.url("/stories/" + story.ID + "/comments"))
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		c.Assert(err, qt.IsNil)

		action, ok := doc.Find("form.new-comment-form").First().Attr("action")
		c.Assert(ok, qt.IsTrue)

		values := url.Values{
			"body":      []string{"very insightful comment"},
			"parent-id": []string{""},
		}

		resp, err = client.PostForm(tc.url(action), values)
		c.Assert(err, qt.IsNil)
		c.Assert(resp.StatusCode, qt.Equals, 200)
		defer resp.Body.Close()

		c.Assert(seen, qt.IsTrue)
	})
}
