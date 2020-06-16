package tabloid

// TODO move template loading into an init func?

import (
	"context"
	"database/sql"
	"encoding/gob"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid/authentication"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"
)

const (
	sessionKey = "tabloid-session"
)

type Server struct {
	Logger          zerolog.Logger
	addr            string
	store           Store
	dbString        string
	router          *httprouter.Router
	done            chan struct{}
	idleConnsClosed chan struct{}
	sessionStore    *sessions.CookieStore
	authService     authentication.AuthService
}

func init() {
	// be able to serialize session data in a cookie
	gob.Register(&oauth2.Token{})
}

func NewServer(addr string, logger zerolog.Logger, store Store, authService authentication.AuthService) *Server {
	return &Server{
		addr:            addr,
		store:           store,
		authService:     authService,
		router:          httprouter.New(),
		Logger:          logger,
		done:            make(chan struct{}),
		idleConnsClosed: make(chan struct{}),
	}
}

func (s *Server) Prepare() error {
	// database
	err := s.store.Connect()
	if err != nil {
		return err
	}

	// routes
	s.router.GET("/", s.HandleIndex())
	s.router.GET("/oauth/start", s.HandleOAuthStart())
	s.router.GET("/oauth/authorize", s.HandleOAuthCallback())
	s.router.GET("/oauth/destroy", s.HandleOAuthDestroy())
	s.router.ServeFiles("/static/*filepath", http.Dir("assets/static"))
	s.router.GET("/submit", s.HandleSubmit())
	s.router.POST("/submit", s.HandleSubmitAction())
	s.router.GET("/stories/:id/comments", s.HandleShow())
	s.router.POST("/stories/:id/comments", s.HandleSubmitCommentAction())

	return nil
}

func (s *Server) Start() error {
	httpServer := http.Server{Addr: s.addr, Handler: s}

	go func() {
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
	s.router.ServeHTTP(res, req)
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
		s.authService.Callback(res, req)

		u, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Fatal().Err(err)
		}

		err = s.store.CreateOrUpdateUser(u.Login, "email")
		if err != nil {
			// TODO dirty
			s.Logger.Fatal().Err(err)
		}
	}
}

func (s *Server) HandleOAuthDestroy() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		s.authService.Destroy(res, req)
	}
}

func (s *Server) HandleIndex() httprouter.Handle {
	tmpl, err := template.ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal().Err(err).Msg("Failed to load templates")
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
			http.Error(res, "Failed to fetch session data", http.StatusInternalServerError)
			return
		}

		res.Header().Set("Content-Type", "text/html")

		if req.Method != "GET" {
			http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
			return
		}

		stories, err := s.store.ListStories()
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to list stories")
			http.Error(res, "Failed to list stories", http.StatusInternalServerError)
			return
		}

		vars := map[string]interface{}{
			"Stories": stories,
			"Session": data,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to render template")
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) HandleSubmit() httprouter.Handle {
	tmpl, err := template.ParseFiles("assets/templates/submit.html", "assets/templates/_header.html", "assets/templates/_footer.html")
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

		// redirect if unathenticated
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
	tmpl, err := template.ParseFiles(
		"assets/templates/show.html",
		"assets/templates/_story.html",
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

		id := params.ByName("id")
		story, err := s.store.FindStory(id)
		if err != nil {
			s.Logger.Error().Err(err).Str("id", id).Msg("Failed to find story")
			// TODO deal with 404
			http.Error(res, "Failed to find story", http.StatusInternalServerError)
		}

		comments, err := s.store.ListComments(strconv.Itoa(int(story.ID)))
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to list comments")
			http.Error(res, "Failed to list comments", http.StatusInternalServerError)
			return
		}

		commentsTree := NewCommentPresentersTree(comments)

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
}

func (s *Server) HandleSubmitAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		data, err := s.CurrentUser(req)
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to fetch Github user")
			http.Error(res, "Failed to fetch Github data", http.StatusMethodNotAllowed)
			return
		}
		// redirect if unathenticated
		if data == nil {
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		err = req.ParseForm()
		if err != nil {
			s.Logger.Warn().Err(err).Msg("Failed to parse form")
			http.Error(res, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// handle form
		title := req.Form["title"][0]
		body := strings.TrimSpace(req.Form["body"][0])
		url := req.Form["url"][0]

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

		story := &Story{
			AuthorID: userRecord.ID,
			Title:    title,
			Body:     body,
			URL:      url,
		}

		err = s.store.InsertStory(story)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to insert story")
			http.Error(res, "Cannot insert story", http.StatusMethodNotAllowed)
			return
		}

		http.Redirect(res, req, "/", http.StatusFound)
	}
}

func (s *Server) HandleSubmitCommentAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")
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
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch current user", http.StatusInternalServerError)
			return
		}

		userRecord, err := s.store.FindUserByLogin(userSession.Login)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
			http.Error(res, "Failed to fetch user from database", http.StatusInternalServerError)
			return
		}

		var comment *Comment
		body := strings.TrimSpace(req.Form["body"][0])
		_parentCommentID := req.Form["parent-id"][0]

		// if not top-level comment
		if _parentCommentID != "" {
			parentCommentID, err := strconv.Atoi(_parentCommentID)
			if err != nil {
				s.Logger.Warn().Err(err).Str("parentID", _parentCommentID).Msg("Failed to parse parent id")
				http.Error(res, "Cannot parse parent ID", http.StatusUnprocessableEntity)
				return
			}
			comment = NewComment(story.ID, sql.NullInt64{Int64: int64(parentCommentID), Valid: true}, body, userRecord.ID)
		} else {
			comment = NewComment(story.ID, sql.NullInt64{Int64: 0, Valid: false}, body, userRecord.ID)
		}

		err = s.store.InsertComment(comment)
		if err != nil {
			s.Logger.Error().Err(err).Msg("Failed to insert comment")
			http.Error(res, "Failed to insert comment", http.StatusMethodNotAllowed)
			return
		}

		storyPath := fmt.Sprintf("/stories/%v/comments", story.ID)
		http.Redirect(res, req, storyPath, http.StatusFound)
	}
}

func (s *Server) CurrentUser(req *http.Request) (*authentication.User, error) {
	return s.authService.CurrentUser(req)
}
