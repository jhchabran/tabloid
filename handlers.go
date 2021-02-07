package tabloid

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jhchabran/tabloid/authentication"
	"github.com/jhchabran/tabloid/ranking"
	"github.com/julienschmidt/httprouter"
)

// HandleE is a httprouter.Handle that also returns an error.
type HandleE func(http.ResponseWriter, *http.Request, httprouter.Params) error

// HandleOAuthStart handles requests starting the OAauth authentication process.
func (s *Server) HandleOAuthStart() HandleE {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) error {
		return s.authService.Start(res, req)
	}
}

// HandleOAuthCallback handles requests of the OAuth provider redirects the user back
// to Tabloid, after successfully authenticating him on its side.
func (s *Server) HandleOAuthCallback() HandleE {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) error {
		// need to think about error handling here
		// probably a before write callback is good enough?
		return s.authService.Callback(res, req, func(u *authentication.User) error {
			_, err := s.store.CreateOrUpdateUser(u.Login, u.Email)
			SetFlash(res, "success", "Signed in.")
			return err
		})
	}
}

// HandleOAuthDestroy handles requests destroying the current session.
func (s *Server) HandleOAuthDestroy() HandleE {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) error {
		return s.authService.Destroy(res, req)
	}
}

// HandleIndex handles requests for the root path, listing sorted paginated stories.
// If the client isn't authenticated, it serves a template with no upvoting nor commenting
// capabilities.
func (s *Server) HandleIndex() HandleE {
	tmpl, err := template.New("index.html").Funcs(helpers).ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to load templates")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		session := ctxSession(req.Context())

		if session != nil {
			s.Logger.Debug().Msg("Authenticated")
			return s.handleAuthenticatedIndex(res, req, params, tmpl)
		} else {
			s.Logger.Debug().Msg("Unauthenticated")
			return s.handleUnauthenticatedIndex(res, req, params, tmpl)
		}
	}
}

// handleAuthenticatedIndex handles requests for when the user is authenticated, notably showing its previous votes.
func (s *Server) handleAuthenticatedIndex(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) error {
	session := ctxSession(req.Context())

	userRecord, err := s.store.FindUserByLogin(session.Login)
	if err != nil {
		return err
	}

	if userRecord == nil {
		// there is a session but no user in the database, wiping the session
		err := s.authService.Destroy(res, req)
		if err != nil {
			return err
		}

		http.Redirect(res, req, "/", http.StatusFound)
		return nil
	}

	res.Header().Set("Content-Type", "text/html")

	var page int
	rawPage, ok := req.URL.Query()["page"]
	if ok && len(rawPage) > 0 {
		page, _ = strconv.Atoi(rawPage[0])
	}

	stories, err := s.store.ListStoriesWithVotes(userRecord.ID, page, s.config.StoriesPerPage)
	if err != nil {
		return err
	}

	// sort story by their rank
	sort.Slice(stories, func(i, j int) bool {
		return rank(stories[i]) > rank(stories[j])
	})

	storyPresenters := []*storyPresenter{}
	for i, st := range stories {
		pos := 1 + i + (page * s.config.StoriesPerPage)
		pr := newStoryPresenterWithPos(&st.Story, pos)
		if st.Up.Valid && st.Up.Bool {
			pr.Upvoted = true
		} else {
			pr.Upvoted = false
		}
		storyPresenters = append(storyPresenters, pr)
	}

	vars := map[string]interface{}{
		"Stories":  storyPresenters,
		"Session":  session,
		"NextPage": page + 1,
		"PrevPage": page - 1,
		"CurrPage": page,
	}

	// HACK, not very elegant but does the job
	nextPageStories, err := s.store.ListStories(page+1, s.config.StoriesPerPage)
	if err != nil {
		return err
	}

	if len(nextPageStories) == 0 {
		vars["NextPage"] = -1
	}

	err = tmpl.Execute(res, vars)
	if err != nil {
		return err
	}

	return nil
}

// handleUnauthenticatedIndex handles requests for when the current user is not authenticated.
func (s *Server) handleUnauthenticatedIndex(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) error {
	res.Header().Set("Content-Type", "text/html")

	var page int
	rawPage, ok := req.URL.Query()["page"]
	if ok && len(rawPage) > 0 {
		page, _ = strconv.Atoi(rawPage[0])
	}

	stories, err := s.store.ListStories(page, s.config.StoriesPerPage)
	if err != nil {
		return err
	}

	// sort story by their rank
	sort.Slice(stories, func(i, j int) bool {
		return rank(stories[i]) > rank(stories[j])
	})

	storyPresenters := []*storyPresenter{}
	for i, st := range stories {
		pos := 1 + i + (page * s.config.StoriesPerPage)
		storyPresenters = append(storyPresenters, newStoryPresenterWithPos(st, pos))
	}

	vars := map[string]interface{}{
		"Stories":  storyPresenters,
		"Session":  nil,
		"NextPage": page + 1,
		"PrevPage": page - 1,
	}

	// HACK, not very elegant but does the job
	nextPageStories, err := s.store.ListStories(page+1, s.config.StoriesPerPage)
	if err != nil {
		return err
	}

	if len(nextPageStories) == 0 {
		vars["NextPage"] = -1
	}

	err = tmpl.Execute(res, vars)
	if err != nil {
		return err
	}

	return nil
}

// HandleSubmit handles requests to get the form for submitting a Story. It redirects to the root path if
// not authenticated.
func (s *Server) HandleSubmit() HandleE {
	tmpl, err := template.New("submit.html").Funcs(helpers).ParseFiles(
		"assets/templates/submit.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to parse template")
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) error {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())

		// redirect if unauthenticated
		if session == nil {
			http.Redirect(res, req, "/", http.StatusFound)
			return nil
		}

		vars := map[string]interface{}{
			"Session": session,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			return err
		}

		return nil
	}
}

// HandleShow handles requests to access a particular Story, showing all its comments and allowing the user to comment
// if authenticated.
func (s *Server) HandleShow() HandleE {
	tmpl, err := template.New("show.html").Funcs(helpers).ParseFiles(
		"assets/templates/show.html",
		"assets/templates/_story_comments.html",
		"assets/templates/_comment.html",
		"assets/templates/_comment_form.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to load template")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())

		if session != nil {
			s.Logger.Debug().Msg("authenticated")
			return s.handleShowAuthenticated(res, req, params, tmpl)
		} else {
			s.Logger.Debug().Msg("unauthenticated")
			return s.handleShowUnauthenticated(res, req, params, tmpl)
		}
	}
}

func (s *Server) handleShowUnauthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) error {
	session := ctxSession(req.Context())

	id := params.ByName("id")
	story, err := s.store.FindStory(id)
	if err != nil {
		return Maybe404(err)
	}

	comments, err := s.store.ListComments(story.ID)
	if err != nil {
		return err
	}

	cc := make([]CommentAccessor, len(comments))
	for i, c := range comments {
		cc[i] = c
	}

	commentsTree := NewCommentPresentersTree(cc)
	commentsTree.Sort(rank)

	err = tmpl.Execute(res, map[string]interface{}{
		"Story":    story,
		"Comments": commentsTree,
		"Session":  session,
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *Server) handleShowAuthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) error {
	session := ctxSession(req.Context())
	userRecord, err := s.store.FindUserByLogin(session.Login)
	if err != nil {
		return err
	}

	if userRecord == nil {
		// there is a session but no user in the database, wiping the session
		err := s.authService.Destroy(res, req)
		if err != nil {
			return err
		}

		http.Redirect(res, req, "/", http.StatusFound)
		return nil
	}

	id := params.ByName("id")
	story, err := s.store.FindStoryWithVote(id, userRecord.ID)
	if err != nil {
		return Maybe404(err)
	}

	comments, err := s.store.ListCommentsWithVotes(story.ID, userRecord.ID)
	if err != nil {
		return err
	}

	cc := make([]CommentAccessor, len(comments))
	for i, c := range comments {
		cc[i] = c
	}
	commentsTree := NewCommentPresentersTree(cc)
	commentsTree.SetCanEdits(userRecord.Name, time.Duration(s.config.EditWindowInMinutes)*time.Minute, NowFunc())
	storyPresenter := newStoryPresenterWithBody(&story.Story)
	storyPresenter.Upvoted = story.Up.Bool

	err = tmpl.Execute(res, map[string]interface{}{
		"Story":    storyPresenter,
		"Comments": commentsTree,
		"Session":  session,
	})

	if err != nil {
		return err
	}

	return nil
}

// HandleSubmitAction handles requests for when a user submit a Story form. It redirects the user to the root path if not
// authenticated. In case someone bypass the client-side form validations with invalid form data,
// it returns a HTTP error.
func (s *Server) HandleSubmitAction() HandleE {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) error {
		res.Header().Set("Content-Type", "text/html")

		err := req.ParseForm()
		if err != nil {
			return BadRequest(err)
		}

		title := strings.TrimSpace(req.FormValue("title"))
		body := strings.TrimSpace(req.FormValue("body"))
		url_ := strings.TrimSpace(req.FormValue("url"))
		if url_ != "" {
			u, err := url.Parse(url_)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return UnprocessableEntityWithError(err, "url", url_)
			}
		}

		if title == "" || len(title) > 64 {
			return UnprocessableEntity("title")
		}

		if url_ == "" && body == "" {
			return UnprocessableEntity("url", "body")
		}

		userRecord := ctxUser(req.Context())
		story := NewStory(title, body, userRecord.ID, url_)

		err = s.store.InsertStory(story)
		if err != nil {
			return err
		}

		story.Author = userRecord.Name

		// HACK
		for _, h := range s.storyHooks {
			err := h(story)
			if err != nil {
				return err
			}
		}

		http.Redirect(res, req, "/stories/"+story.ID+"/comments", http.StatusFound)
		return nil
	}
}

// HandleSubmitCommentAction handles requests for when a user submit a Comment form for a given Story. It redirects
// the user to the root path if not authenticated. In case someone bypass the client-side form validations
// with invalid form data, it returns a HTTP error.
func (s *Server) HandleSubmitCommentAction() HandleE {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())
		// redirect if unauthenticated
		if session == nil {
			return Unauthorized(req.URL.Path)
		}

		id := params.ByName("id")
		story, err := s.store.FindStory(id)
		if err != nil {
			return Maybe404(err)
		}

		err = req.ParseForm()
		if err != nil {
			return BadRequest(err)
		}

		userRecord := ctxUser(req.Context())

		var comment *Comment
		body := strings.TrimSpace(req.FormValue("body"))
		parentCommentID := req.FormValue("parent-id")

		// if not top-level comment
		if parentCommentID != "" {
			comment = NewComment(story.ID, sql.NullString{String: parentCommentID, Valid: true}, body, userRecord.ID)
		} else {
			comment = NewComment(story.ID, sql.NullString{String: "", Valid: false}, body, userRecord.ID)
		}

		err = s.store.InsertComment(comment)
		if err != nil {
			return err
		}

		// HACK
		comment.Author = userRecord.Name
		for _, h := range s.commentHooks {
			err := h(story, comment)
			if err != nil {
				s.Logger.Warn().Err(err).Msg("story hook failed")
				return err
			}
		}

		storyPath := fmt.Sprintf("/stories/%v/comments", story.ID)
		http.Redirect(res, req, storyPath, http.StatusFound)
		return nil
	}
}

// HandleVoteCommentAction handles requests to vote on a comment. It redirects back to the Story on which
// the Comment was posted on. If not authenticated, it redirects to the root path.
func (s *Server) HandleVoteCommentAction() HandleE {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		// We'll redirect to a given route after submitting this, so we use redir to specify it
		redir, err := normalizeRedir(req.URL.Query()["redir"])
		if err != nil {
			return UnprocessableEntityWithError(err, "redir")
		}

		storyID := params.ByName("story_id")
		_, err = s.store.FindStory(storyID)
		if err != nil {
			return Maybe404(err)
		}

		id := params.ByName("id")
		s.Logger.Debug().Str("id", id).Msg("comment")
		_, err = s.store.FindComment(id)
		if err != nil {
			return Maybe404(err)
		}

		userRecord := ctxUser(req.Context())

		err = s.store.CreateOrUpdateVoteOnComment(id, userRecord.ID, true)
		if err != nil {
			return err
		}

		http.Redirect(res, req, redir, http.StatusFound)
		return nil
	}
}

// HandleVoteStoryAction handles requests to vote on a given Story. If not authenticated, it redirects to the root path.
func (s *Server) HandleVoteStoryAction() HandleE {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		// We'll redirect to a given route after submitting this, so we use redir to specify it
		redir, err := normalizeRedir(req.URL.Query()["redir"])
		if err != nil {
			return UnprocessableEntityWithError(err, "redir")
		}

		id := params.ByName("id")
		_, err = s.store.FindStory(id)
		if err != nil {
			return Maybe404(err)
		}

		userRecord := ctxUser(req.Context())
		if err != nil {
			return err
		}

		err = s.store.CreateOrUpdateVoteOnStory(id, userRecord.ID, true)
		if err != nil {
			return err
		}

		http.Redirect(res, req, redir, http.StatusFound)
		return nil
	}
}

func (s *Server) HandleCommentEdit() HandleE {
	tmpl, err := template.New("edit.html").Funcs(helpers).ParseFiles(
		"assets/templates/edit.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to parse template")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		res.Header().Set("Content-Type", "text/html")
		session := ctxSession(req.Context())
		userRecord := ctxUser(req.Context())

		storyID := params.ByName("story_id")
		_, err := s.store.FindStory(storyID)
		if err != nil {
			return Maybe404(err)
		}

		story, err := s.store.FindStory(storyID)
		if err != nil {
			return Maybe404(err)
		}

		id := params.ByName("id")
		comment, err := s.store.FindComment(id)
		if err != nil {
			return Maybe404(err)
		}

		if err != nil {
			return err
		}

		// Cannot edit comments that aren't yours.
		if comment.AuthorID != userRecord.ID {
			http.Redirect(res, req, "/", http.StatusFound)
			return nil
		}

		// If comment is older than edit window, let's redirect
		editWindow := time.Duration(s.config.EditWindowInMinutes) * time.Minute
		if comment.CreatedAt.Add(editWindow).Before(NowFunc()) {
			SetFlash(res, "warning", "Comment is too old to be edited.")
			http.Redirect(res, req, "/stories/"+story.ID+"/comments", http.StatusFound)
			return nil
		}

		vars := map[string]interface{}{
			"Session": session,
			"Comment": comment,
			"Story":   story,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			return err
		}

		return nil
	}
}

func (s *Server) HandleCommentUpdateAction() HandleE {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) error {
		userRecord := ctxUser(req.Context())

		storyID := params.ByName("story_id")
		_, err := s.store.FindStory(storyID)
		if err != nil {
			return Maybe404(err)
		}

		id := params.ByName("id")
		comment, err := s.store.FindComment(id)

		if err != nil {
			return Maybe404(err)
		}

		if err != nil {
			return err
		}

		// Cannot edit comments that aren't yours.
		if comment.AuthorID != userRecord.ID {
			http.Redirect(res, req, "/", http.StatusFound)
			return nil
		}

		err = req.ParseForm()
		if err != nil {
			return UnprocessableEntityWithError(err)
		}

		comment.Body = req.Form.Get("body")
		err = s.store.UpdateComment(comment)
		if err != nil {
			return err
		}

		SetFlash(res, "success", "Comment has been edited")
		http.Redirect(res, req, "/stories/"+storyID+"/comments", http.StatusFound)
		return nil
	}
}

func rank(s ranking.Rankable) float64 {
	return ranking.Rank(s, 1.8, 4*24, NowFunc())
}
