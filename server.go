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

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid/authentication"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"golang.org/x/oauth2"
)

const (
	sessionKey = "tabloid-session"
)

var NowFunc func() time.Time = time.Now

var helpers template.FuncMap = template.FuncMap{
	"daysAgo": func(t time.Time) string {
		now := time.Now()
		days := int(now.Sub(t).Hours() / 24)

		if days < 1 {
			return "today"
		}
		return strconv.Itoa(days) + " days ago"
	},
	"title": strings.Title,
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("invalid dict call, odd number of arguments")
		}

		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			k, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			v := values[i+1]
			dict[k] = v
		}

		return dict, nil
	},
}

type middleware func(http.Handler) http.Handler
type StoryHookFn func(*Story) error
type CommentHookFn func(*Story, *Comment) error

type Server struct {
	Logger          zerolog.Logger
	config          *ServerConfig
	store           Store
	dbString        string
	sessionStore    *sessions.CookieStore
	router          *httprouter.Router
	authService     authentication.AuthService
	middlewares     []middleware
	rootHandler     http.Handler
	done            chan struct{}
	idleConnsClosed chan struct{}
	storyHooks      []StoryHookFn
	commentHooks    []CommentHookFn
}

type ServerConfig struct {
	Addr           string
	StoriesPerPage int
}

func init() {
	// be able to serialize session data in a cookie
	gob.Register(&oauth2.Token{})
}

func NewServer(config *ServerConfig, logger zerolog.Logger, store Store, authService authentication.AuthService) *Server {
	middlewares := []middleware{
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

	s := &Server{
		config:          config,
		store:           store,
		authService:     authService,
		router:          httprouter.New(),
		Logger:          logger,
		done:            make(chan struct{}),
		idleConnsClosed: make(chan struct{}),
	}

	var h http.Handler = s.router
	for _, m := range middlewares {
		h = m(h)
	}

	s.rootHandler = h

	return s
}

func (s *Server) Prepare() error {
	// database
	s.Logger.Debug().Msg("connecting to database")
	err := s.store.Connect()
	if err != nil {
		return err
	}

	// routes
	s.Logger.Debug().Msg("declaring routes")
	s.router.GET("/", s.HandleIndex())
	s.router.GET("/oauth/start", s.HandleOAuthStart())
	s.router.GET("/oauth/authorize", s.HandleOAuthCallback())
	s.router.GET("/oauth/destroy", s.HandleOAuthDestroy())
	s.router.ServeFiles("/static/*filepath", http.Dir("assets/static"))
	s.router.GET("/submit", s.HandleSubmit())
	s.router.POST("/submit", s.HandleSubmitAction())
	s.router.GET("/stories/:id/comments", s.HandleShow())
	s.router.POST("/stories/:id/comments", s.HandleSubmitCommentAction())
	s.router.POST("/stories/:id/votes", s.HandleVoteStoryAction())
	// httprouter conflicts if we use /stories/:story_id, waiting on httprouter v2
	// https://github.com/julienschmidt/httprouter/issues/175#issuecomment-270075906
	s.router.POST("/story/:story_id/comments/:id/votes", s.HandleVoteCommentAction())

	return nil
}

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

func (s *Server) Stop() {
	close(s.done)
	<-s.idleConnsClosed
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.rootHandler.ServeHTTP(res, req)
}

func (s *Server) HandleOAuthStart() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		s.authService.Start(res, req)
	}
}

func (s *Server) HandleOAuthCallback() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		// need to think about error handling here
		// probably a before write callback is good enough?
		s.authService.Callback(res, req, func(u *authentication.User) error {
			_, err := s.store.CreateOrUpdateUser(u.Login, "email")
			return err
		})
	}
}

func (s *Server) HandleOAuthDestroy() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		s.authService.Destroy(res, req)
	}
}

func (s *Server) CurrentUser(req *http.Request) (*authentication.User, error) {
	return s.authService.CurrentUser(req)
}

type storyPresenter struct {
	Pos           int
	ID            string
	Title         string
	URL           string
	Body          string
	Score         int
	Author        string
	AuthorID      string
	CommentsCount int64
	CreatedAt     time.Time
	Upvoted       bool
}

func newStoryPresenter(story *Story, pos int) *storyPresenter {
	return &storyPresenter{
		Pos:           pos,
		ID:            story.ID,
		Title:         story.Title,
		URL:           story.URL,
		Body:          story.Body,
		Score:         story.Score,
		Author:        story.Author,
		AuthorID:      story.AuthorID,
		CommentsCount: story.CommentsCount,
		CreatedAt:     story.CreatedAt,
	}
}

func (s *Server) HandleIndex() httprouter.Handle {
	tmpl, err := template.New("index.html").Funcs(helpers).ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to load templates")
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
			http.Error(res, "Failed to fetch session data", http.StatusInternalServerError)
			return
		}

		if data != nil {
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

func (s *Server) handleAuthenticatedIndex(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	data, err := s.CurrentUser(req)
	if err != nil {
		s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
		http.Error(res, "Failed to fetch session data", http.StatusInternalServerError)
		return
	}

	userRecord, err := s.store.FindUserByLogin(data.Login)
	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
		http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
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
		pr := newStoryPresenter(&st.Story, pos)
		if st.Up.Valid && st.Up.Bool {
			pr.Upvoted = true
		} else {
			pr.Upvoted = false
		}
		storyPresenters = append(storyPresenters, pr)
	}

	vars := map[string]interface{}{
		"Stories":  storyPresenters,
		"Session":  data,
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
	s.Logger.Warn().Msgf("len=%v", len(nextPageStories))
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
		storyPresenters = append(storyPresenters, newStoryPresenter(st, pos))
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

		userSession, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
			http.Error(res, "Failed to fetch session data", http.StatusMethodNotAllowed)
			return
		}

		// redirect if unauthenticated
		if userSession == nil {
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		vars := map[string]interface{}{
			"Session": userSession,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to render template")
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

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

		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch Github user")
			http.Error(res, "Failed to fetch Github user", http.StatusMethodNotAllowed)
			return
		}

		if data != nil {
			s.Logger.Debug().Msg("authenticated")
			s.HandleShowAuthenticated(res, req, params, tmpl)
		} else {
			s.Logger.Debug().Msg("unauthenticated")
			s.HandleShowUnauthenticated(res, req, params, tmpl)
		}
	}
}

func (s *Server) HandleShowUnauthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	data, err := s.CurrentUser(req)
	if err != nil {
		s.Logger.Warn().Err(err).Msg("Failed to fetch Github user")
		http.Error(res, "Failed to fetch Github user", http.StatusMethodNotAllowed)
		return
	}

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
		"Session":  data,
	})

	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleShowAuthenticated(res http.ResponseWriter, req *http.Request, params httprouter.Params, tmpl *template.Template) {
	data, err := s.CurrentUser(req)
	if err != nil {
		s.Logger.Warn().Err(err).Msg("Failed to fetch Github user")
		http.Error(res, "Failed to fetch Github user", http.StatusMethodNotAllowed)
		return
	}

	id := params.ByName("id")
	story, err := s.store.FindStory(id)
	if err != nil {
		s.Logger.Error().Err(err).Str("id", id).Msg("Failed to find story")
		// TODO deal with 404
		http.Error(res, "Failed to find story", http.StatusInternalServerError)
	}

	userRecord, err := s.store.FindUserByLogin(data.Login)
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

	err = tmpl.Execute(res, map[string]interface{}{
		"Story":    story,
		"Comments": commentsTree,
		"Session":  data,
	})

	if err != nil {
		s.Logger.Error().Err(err).Msg("Failed to render template")
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleSubmitAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failuped to fetch Github user")
			http.Error(res, "Failed to fetch Github data", http.StatusMethodNotAllowed)
			return
		}

		if data == nil {
			http.Error(res, "Unauthorized", http.StatusUnauthorized)
			return
		}

		err = req.ParseForm()
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

		// grab author stuff
		userSession, err := s.authService.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch Github user")
			http.Error(res, "Failed to fetch current user", http.StatusInternalServerError)
			return
		}

		userRecord, err := s.store.FindUserByLogin(userSession.Login)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		story := NewStory(title, body, userRecord.ID, url_)

		err = s.store.InsertStory(story)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to insert story")
			http.Error(res, "Cannot insert story", http.StatusMethodNotAllowed)
			return
		}

		story.Author = userSession.Login

		// HACK
		for _, h := range s.storyHooks {
			err := h(story)
			if err != nil {
				http.Error(res, "hook failed", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(res, req, "/stories/"+story.ID+"/comments", http.StatusFound)
	}
}

func (s *Server) HandleSubmitCommentAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failuped to fetch Github user")
			http.Error(res, "Failed to fetch Github data", http.StatusMethodNotAllowed)
			return
		}
		// redirect if unauthenticated
		if data == nil {
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

		// prepare the user
		userSession, err := s.authService.CurrentUser(req)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to authenticate user")
			http.Error(res, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		userRecord, err := s.store.FindUserByLogin(userSession.Login)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

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
		comment.Author = userSession.Login
		for _, h := range s.commentHooks {
			err := h(story, comment)
			if err != nil {
				http.Error(res, "hook failed", http.StatusInternalServerError)
				return
			}
		}

		storyPath := fmt.Sprintf("/stories/%v/comments", story.ID)
		http.Redirect(res, req, storyPath, http.StatusFound)
	}
}

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
			s.Logger.Error().Err(err).Msg("Failed to find story")
			http.Error(res, "Failed to find story", http.StatusNotFound)
			return
		}

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

		userSession, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to authenticate user")
			http.Error(res, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		// TODO deal with user not being authenticated

		userRecord, err := s.store.FindUserByLogin(userSession.Login)
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
		return
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

		userSession, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to authenticate user")
			http.Error(res, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		userRecord, err := s.store.FindUserByLogin(userSession.Login)
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
		return
	}
}

func (s *Server) AddStoryHook(fn StoryHookFn) {
	s.storyHooks = append(s.storyHooks, fn)
}

func (s *Server) AddCommentHook(fn CommentHookFn) {
	s.commentHooks = append(s.commentHooks, fn)
}
