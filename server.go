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

// StoryHookFn represents a function suitable for Story hooks
type StoryHookFn func(*Story) error

// CommentHookFn represents a function suitable for Commet hooks
type CommentHookFn func(*Story, *Comment) error

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

func (sp *storyPresenter) IsSelfPost() bool {
	return sp.URL == ""
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
