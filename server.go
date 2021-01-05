package tabloid

// TODO move template loading into an init func?

import (
	"context"
	"database/sql"
	"encoding/gob"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jhchabran/tabloid/authentication"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"golang.org/x/oauth2"
)

// StoryHookFn represents a function suitable for Story hooks
type StoryHookFn func(*Story) error

// CommentHookFn represents a function suitable for Commet hooks
type CommentHookFn func(*Story, *Comment) error

// Server represents the HTTP server component, with all its runtime dependencies.
type Server struct {
	// Logger is the server logger
	Logger          zerolog.Logger
	config          *ServerConfig
	store           Store
	router          *httprouter.Router
	authService     authentication.AuthService
	rootHandler     http.Handler
	done            chan struct{}
	idleConnsClosed chan struct{}
	storyHooks      []StoryHookFn
	commentHooks    []CommentHookFn
}

// ServerConfig represents the settings required for the server to operate.
type ServerConfig struct {
	Addr                string
	StoriesPerPage      int
	EditWindowInMinutes int
}

func init() {
	// be able to serialize session data in a cookie
	gob.Register(&oauth2.Token{})
}

// NewServer returns a server instance, configured with given components and with middlewares installed.
func NewServer(config *ServerConfig, logger zerolog.Logger, store Store, authService authentication.AuthService) *Server {
	s := &Server{
		config:          config,
		store:           store,
		authService:     authService,
		router:          httprouter.New(),
		Logger:          logger,
		done:            make(chan struct{}),
		idleConnsClosed: make(chan struct{}),
	}

	// Those are top level middewares, set before the router; every requests will go through them.
	middlewares := []httpMiddleware{
		s.httpVerbFormUnwrapper,
		hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
			hlog.FromRequest(r).Info().
				Str("method", r.Method).
				Stringer("url", r.URL).
				Int("status", status).
				Int("size", size).
				Dur("duration", duration).
				Msg("")
		}),
		hlog.NewHandler(logger),
	}

	var h http.Handler = s.router
	for _, m := range middlewares {
		h = m(h)
	}

	s.rootHandler = h

	return s
}

// Prepare setups all internal components, like connecting to the database, declaring routes and loading templates.
func (s *Server) Prepare() error {
	// database
	s.Logger.Debug().Msg("connecting to database")
	err := s.store.Connect()
	if err != nil {
		return err
	}

	// routes
	s.Logger.Debug().Msg("declaring routes")

	s.router.GET("/oauth/start", s.HandleOAuthStart())
	s.router.GET("/oauth/authorize", s.HandleOAuthCallback())
	s.router.GET("/oauth/destroy", s.HandleOAuthDestroy())

	withMiddlewares(func(m middleware) {
		s.router.GET("/", m(s.HandleIndex()))
		s.router.GET("/stories/:id/comments", m(s.HandleShow()))
		s.router.GET("/submit", m(s.HandleSubmit()))
	}, s.loadSessionMiddleware())

	withMiddlewares(func(m middleware) {
		s.router.POST("/submit", m(s.HandleSubmitAction()))
		s.router.POST("/stories/:id/comments", m(s.HandleSubmitCommentAction()))
		s.router.POST("/stories/:id/votes", m(s.HandleVoteStoryAction()))
		s.router.POST("/story/:story_id/comments/:id/votes", m(s.HandleVoteCommentAction()))
		s.router.GET("/story/:story_id/comments/:id/edit", m(s.HandleCommentEdit()))
		s.router.PUT("/story/:story_id/comments/:id", m(s.HandleCommentUpdateAction()))
	}, s.loadSessionMiddleware(), s.loadUserMiddleware())

	s.router.ServeFiles("/static/*filepath", http.Dir("assets/static"))

	return nil
}

// Start runs the server and will block until stopped.
func (s *Server) Start() error {
	httpServer := http.Server{Addr: s.config.Addr, Handler: s}

	go func() {
		s.Logger.Debug().Str("addr", s.config.Addr).Msg("running server")
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			// should probably bubble this up
			s.Logger.Fatal().Err(err)
		}
	}()

	<-s.done

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return err
	}
	close(s.idleConnsClosed)

	return nil
}

// Stop gracefully stops a running server.
func (s *Server) Stop() {
	close(s.done)
	<-s.idleConnsClosed
}

// ServeHTTP implements a http.Handler that answers incoming requests.
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.rootHandler.ServeHTTP(res, req)
}

// HandleOAuthStart handles requests starting the OAauth authentication process.
func (s *Server) HandleOAuthStart() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		s.authService.Start(res, req)
	}
}

// HandleOAuthCallback handles requests of the OAuth provider redirects the user back
// to Tabloid, after successfully authenticating him on its side.
func (s *Server) HandleOAuthCallback() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		// need to think about error handling here
		// probably a before write callback is good enough?
		s.authService.Callback(res, req, func(u *authentication.User) error {
			_, err := s.store.CreateOrUpdateUser(u.Login, u.Email)
			return err
		})
	}
}

// HandleOAuthDestroy handles requests destroying the current session.
func (s *Server) HandleOAuthDestroy() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		s.authService.Destroy(res, req)
	}
}

// HandleIndex handles requests for the root path, listing sorted paginated stories.
// If the client isn't authenticated, it serves a template with no upvoting nor commenting
// capabilities.
func (s *Server) HandleIndex() httprouter.Handle {
	tmpl, err := template.New("index.html").Funcs(helpers).ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to load templates")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		session := ctxSession(req.Context())

		if session != nil {
			s.Logger.Debug().Msg("Authenticated")
			s.handleAuthenticatedIndex(res, req, params, tmpl)
			return
		} else {
			s.Logger.Debug().Msg("Unauthenticated")
			s.handleUnauthenticatedIndex(res, req, params, tmpl)
			return
		}
	}
}

// handleAuthenticatedIndex handles requests for when the user is authenticated, notably showing its previous votes.
func (s *Server) handleAuthenticatedIndex(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	session := ctxSession(req.Context())

	userRecord, err := s.store.FindUserByLogin(session.Login)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
		http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
		return
	}

	if userRecord == nil {
		// there is a session but no user in the database, wiping the session
		s.authService.Destroy(res, req)
		return
	}

	res.Header().Set("Content-Type", "text/html")

	if req.Method != "GET" {
		http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	var page int
	rawPage, ok := req.URL.Query()["page"]
	if ok && len(rawPage) > 0 {
		page, _ = strconv.Atoi(rawPage[0])
	}

	stories, err := s.store.ListStoriesWithVotes(userRecord.ID, page, s.config.StoriesPerPage)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to list stories")
		http.Error(res, "Failed to list stories", http.StatusInternalServerError)
		return
	}

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
		s.Logger.Error().Err(err).Msg("Failed to list stories")
		http.Error(res, "Failed to list stories", http.StatusInternalServerError)
		return
	}
	if len(nextPageStories) == 0 {
		vars["NextPage"] = -1
	}

	err = tmpl.Execute(res, vars)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

// handleUnauthenticatedIndex handles requests for when the current user is not authenticated.
func (s *Server) handleUnauthenticatedIndex(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	res.Header().Set("Content-Type", "text/html")

	if req.Method != "GET" {
		http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	var page int
	rawPage, ok := req.URL.Query()["page"]
	if ok && len(rawPage) > 0 {
		page, _ = strconv.Atoi(rawPage[0])
	}

	stories, err := s.store.ListStories(page, s.config.StoriesPerPage)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to list stories")
		http.Error(res, "Failed to list stories", http.StatusInternalServerError)
		return
	}

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
		s.Logger.Error().Err(err).Msg("Failed to list stories")
		http.Error(res, "Failed to list stories", http.StatusInternalServerError)
		return
	}

	if len(nextPageStories) == 0 {
		vars["NextPage"] = -1
	}

	err = tmpl.Execute(res, vars)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

// HandleSubmit handles requests to get the form for submitting a Story. It redirects to the root path if
// not authenticated.
func (s *Server) HandleSubmit() httprouter.Handle {
	tmpl, err := template.New("submit.html").Funcs(helpers).ParseFiles(
		"assets/templates/submit.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to parse template")
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())

		// redirect if unauthenticated
		if session == nil {
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		vars := map[string]interface{}{
			"Session": session,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to render template")
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

// HandleShow handles requests to access a particular Story, showing all its comments and allowing the user to comment
// if authenticated.
func (s *Server) HandleShow() httprouter.Handle {
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

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())

		if session != nil {
			s.Logger.Debug().Msg("authenticated")
			s.handleShowAuthenticated(res, req, params, tmpl)
		} else {
			s.Logger.Debug().Msg("unauthenticated")
			s.handleShowUnauthenticated(res, req, params, tmpl)
		}
	}
}

func (s *Server) handleShowUnauthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	session := ctxSession(req.Context())

	id := params.ByName("id")
	story, err := s.store.FindStory(id)
	if err != nil {
		s.Logger.Error().Err(err).Str("id", id).Msg("Failed to find story")
		// TODO deal with 404
		http.Error(res, "Failed to find story", http.StatusInternalServerError)
	}

	comments, err := s.store.ListComments(story.ID)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to list comments")
		http.Error(res, "Failed to list comments", http.StatusInternalServerError)
		return
	}

	cc := make([]CommentAccessor, len(comments))
	for i, c := range comments {
		cc[i] = c
	}
	commentsTree := NewCommentPresentersTree(cc)

	err = tmpl.Execute(res, map[string]interface{}{
		"Story":    story,
		"Comments": commentsTree,
		"Session":  session,
	})

	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleShowAuthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	session := ctxSession(req.Context())

	id := params.ByName("id")
	story, err := s.store.FindStory(id)
	if err != nil {
		s.Logger.Error().Err(err).Str("id", id).Msg("Failed to find story")
		// TODO deal with 404
		http.Error(res, "Failed to find story", http.StatusInternalServerError)
	}

	userRecord, err := s.store.FindUserByLogin(session.Login)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
		http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
		return
	}

	comments, err := s.store.ListCommentsWithVotes(story.ID, userRecord.ID)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to list comments")
		http.Error(res, "Failed to list comments", http.StatusInternalServerError)
		return
	}

	cc := make([]CommentAccessor, len(comments))
	for i, c := range comments {
		cc[i] = c
	}
	commentsTree := NewCommentPresentersTree(cc)
	commentsTree.SetCanEdits(userRecord.Name, time.Duration(s.config.EditWindowInMinutes)*time.Minute, NowFunc())

	err = tmpl.Execute(res, map[string]interface{}{
		"Story":    newStoryPresenterWithBody(story),
		"Comments": commentsTree,
		"Session":  session,
	})

	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

// HandleSubmitAction handles requests for when a user submit a Story form. It redirects the user to the root path if not
// authenticated. In case someone bypass the client-side form validations with invalid form data,
// it returns a HTTP error.
func (s *Server) HandleSubmitAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		err := req.ParseForm()
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to parse form")
			http.Error(res, "Failed to parse form", http.StatusBadRequest)
			return
		}

		title := strings.TrimSpace(req.FormValue("title"))
		body := strings.TrimSpace(req.FormValue("body"))
		url_ := strings.TrimSpace(req.FormValue("url"))
		if url_ != "" {
			u, err := url.Parse(url_)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(res, "", http.StatusBadRequest)
				return
			}
		}

		if title == "" || len(title) > 64 {
			http.Error(res, "", http.StatusBadRequest)
			return
		}

		if url_ == "" && body == "" {
			http.Error(res, "", http.StatusBadRequest)
			return
		}

		userRecord := ctxUser(req.Context())
		story := NewStory(title, body, userRecord.ID, url_)

		err = s.store.InsertStory(story)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to insert story")
			http.Error(res, "Cannot insert story", http.StatusMethodNotAllowed)
			return
		}

		story.Author = userRecord.Name

		// HACK
		for _, h := range s.storyHooks {
			err := h(story)
			if err != nil {
				s.Logger.Warn().Err(err).Msg("story hook failed")
				http.Error(res, "hook failed", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(res, req, "/stories/"+story.ID+"/comments", http.StatusFound)
	}
}

// HandleSubmitCommentAction handles requests for when a user submit a Comment form for a given Story. It redirects
// the user to the root path if not authenticated. In case someone bypass the client-side form validations
// with invalid form data, it returns a HTTP error.
func (s *Server) HandleSubmitCommentAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		session := ctxSession(req.Context())
		// redirect if unauthenticated
		if session == nil {
			http.Error(res, "Must be authenticated to submit comment", http.StatusUnauthorized)
			return
		}

		id := params.ByName("id")
		story, err := s.store.FindStory(id)
		if err != nil {
			s.Logger.Error().Err(err).Str("id", id).Msg("Failed to find story")
			// TODO deal with 404
			http.Error(res, "Failed to find story", http.StatusInternalServerError)
			return
		}

		err = req.ParseForm()
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to parse form")
			http.Error(res, "Failed to parse form", http.StatusBadRequest)
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
			s.Logger.Error().Err(err).Msg("Failed to insert comment")
			http.Error(res, "Failed to insert comment", http.StatusMethodNotAllowed)
			return
		}

		// HACK
		comment.Author = userRecord.Name
		for _, h := range s.commentHooks {
			err := h(story, comment)
			if err != nil {
				s.Logger.Warn().Err(err).Msg("story hook failed")
				http.Error(res, "hook failed", http.StatusInternalServerError)
				return
			}
		}

		storyPath := fmt.Sprintf("/stories/%v/comments", story.ID)
		http.Redirect(res, req, storyPath, http.StatusFound)
	}
}

// HandleVoteCommentAction handles requests to vote on a comment. It redirects back to the Story on which
// the Comment was posted on. If not authenticated, it redirects to the root path.
func (s *Server) HandleVoteCommentAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		// We'll redirect to a given route after submitting this, so we use redir to specify it
		redir, err := normalizeRedir(req.URL.Query()["redir"])
		if err != nil {
			s.Logger.Debug().Err(err).Msg("suspect redir param")
			http.Error(res, "Suspect redirection", http.StatusBadRequest)
			return
		}

		storyID := params.ByName("story_id")
		_, err = s.store.FindStory(storyID)
		if err != nil {
			http.Error(res, "Story Not found", http.StatusNotFound)
			return
		}

		id := params.ByName("id")
		s.Logger.Debug().Str("id", id).Msg("comment")
		_, err = s.store.FindComment(id)
		if err != nil {
			s.Logger.Debug().Err(err).Msg("comment")
			http.Error(res, "Comment Not found", http.StatusNotFound)
			return
		}

		userRecord := ctxUser(req.Context())
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(res, "Wrong format for story id", http.StatusBadRequest)
			return
		}

		err = s.store.CreateOrUpdateVoteOnComment(id, userRecord.ID, true)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to create upvote")
			http.Error(res, "Failed to create upvote", http.StatusInternalServerError)
			return
		}

		http.Redirect(res, req, redir, http.StatusFound)
		s.Logger.Debug().Str("ref", req.Referer()).Msg("ss")
	}
}

// HandleVoteStoryAction handles requests to vote on a given Story. If not authenticated, it redirects to the root path.
func (s *Server) HandleVoteStoryAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		// We'll redirect to a given route after submitting this, so we use redir to specify it
		redir, err := normalizeRedir(req.URL.Query()["redir"])
		if err != nil {
			s.Logger.Debug().Err(err).Msg("suspect redir param")
			http.Error(res, "Suspect redirection", http.StatusBadRequest)
			return
		}

		id := params.ByName("id")
		_, err = s.store.FindStory(id)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to find story")
			http.Error(res, "Failed to find story", http.StatusNotFound)
			return
		}

		userRecord := ctxUser(req.Context())
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		err = s.store.CreateOrUpdateVoteOnStory(id, userRecord.ID, true)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to create upvote")
			http.Error(res, "Failed to create upvote", http.StatusInternalServerError)
			return
		}

		http.Redirect(res, req, redir, http.StatusFound)
	}
}

func (s *Server) HandleCommentEdit() httprouter.Handle {
	tmpl, err := template.New("edit.html").Funcs(helpers).ParseFiles(
		"assets/templates/edit.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to parse template")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")
		session := ctxSession(req.Context())
		userRecord := ctxUser(req.Context())

		storyID := params.ByName("story_id")
		_, err := s.store.FindStory(storyID)
		if err != nil {
			http.Error(res, "Failed to find story", http.StatusNotFound)
			return
		}

		story, err := s.store.FindStory(storyID)
		if err != nil {
			http.Error(res, "Story Not found", http.StatusNotFound)
			return
		}

		id := params.ByName("id")
		comment, err := s.store.FindComment(id)
		if err != nil {
			s.Logger.Debug().Err(err).Msg("comment")
			http.Error(res, "Comment Not found", http.StatusNotFound)
			return
		}

		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		// Cannot edit comments that aren't yours.
		if comment.AuthorID != userRecord.ID {
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		// If comment is older than edit window, let's redirect
		editWindow := time.Duration(s.config.EditWindowInMinutes) * time.Minute
		if comment.CreatedAt.Add(editWindow).Before(NowFunc()) {
			http.Redirect(res, req, "/stories/"+story.ID+"/comments", http.StatusFound)
			return
		}

		vars := map[string]interface{}{
			"Session": session,
			"Comment": comment,
			"Story":   story,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to render template")
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) HandleCommentUpdateAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		userRecord := ctxUser(req.Context())

		storyID := params.ByName("story_id")
		_, err := s.store.FindStory(storyID)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to find story")
			http.Error(res, "Failed to find story", http.StatusNotFound)
			return
		}

		id := params.ByName("id")
		comment, err := s.store.FindComment(id)
		if err != nil {
			s.Logger.Debug().Err(err).Msg("comment")
			http.Error(res, "Comment Not found", http.StatusNotFound)
			return
		}

		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		// Cannot edit comments that aren't yours.
		if comment.AuthorID != userRecord.ID {
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		err = req.ParseForm()
		if err != nil {
			http.Error(res, "Bad Request", http.StatusBadRequest)
			return
		}

		comment.Body = req.Form.Get("body")
		err = s.store.UpdateComment(comment)
		if err != nil {
			s.Logger.Error().Err(err).Msg("can't update comment in db")
			http.Error(res, "Server Error", http.StatusInternalServerError)
			return
		}

		http.Redirect(res, req, "/stories/"+storyID+"/comments", http.StatusFound)
	}
}

// AddStoryHook registers a given StoryHookFn, that will be called every time a story is submitted.
// Multiple hooks will be called in the order they were registered.
// If a hook fails and returns an error, it will interrupt the request but won't prevent the Story to be created.
func (s *Server) AddStoryHook(fn StoryHookFn) {
	s.storyHooks = append(s.storyHooks, fn)
}

// AddCommentHook registers a given CommentHookFn, that will be called every time a story is submitted.
// Multiple hooks will be called in the order they were registered.
// If a hook fails and returns an error, it will interrupt the request but won't prevent the Comment to be created.
func (s *Server) AddCommentHook(fn CommentHookFn) {
	s.commentHooks = append(s.commentHooks, fn)
}

type storyPresenter struct {
	Pos           int
	ID            string
	Title         string
	URL           string
	Body          template.HTML
	Score         int
	Author        string
	AuthorID      string
	CommentsCount int64
	CreatedAt     time.Time
	Upvoted       bool
}

func newStoryPresenterWithPos(story *Story, pos int) *storyPresenter {
	return &storyPresenter{
		Pos:           pos,
		ID:            story.ID,
		Title:         story.Title,
		URL:           story.URL,
		Score:         story.Score,
		Author:        story.Author,
		AuthorID:      story.AuthorID,
		CommentsCount: story.CommentsCount,
		CreatedAt:     story.CreatedAt,
	}
}

func newStoryPresenterWithBody(story *Story) *storyPresenter {
	return &storyPresenter{
		ID:            story.ID,
		Title:         story.Title,
		URL:           story.URL,
		Body:          renderBody(story.Body),
		Score:         story.Score,
		Author:        story.Author,
		AuthorID:      story.AuthorID,
		CommentsCount: story.CommentsCount,
		CreatedAt:     story.CreatedAt,
	}
}

// TODO move this, and test it
func normalizeRedir(redir []string) (string, error) {
	if len(redir) != 1 {
		return "", fmt.Errorf("more than one redir path")
	}
	if redir[0] == "" {
		return "", fmt.Errorf("redir can't be empty")
	}
	if !strings.HasPrefix(redir[0], "/") {
		return "", fmt.Errorf("redir must start with a /")
	}

	return redir[0], nil
}
