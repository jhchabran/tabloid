package tabloid

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"
)

const (
	sessionKey = "tabloid-session"
)

type config struct {
	ClientSecret string `json:"clientSecret"`
	ClientID     string `json:"clientID"`
	ServerSecret string `json:"serverSecret"`
}

type Server struct {
	Logger          *log.Logger
	addr            string
	store           Store
	dbString        string
	router          *httprouter.Router
	done            chan struct{}
	idleConnsClosed chan struct{}
	config          config
	oauthConfig     *oauth2.Config
	sessionStore    *sessions.CookieStore
}

func init() {
	// be able to serialize session data in a cookie
	gob.Register(&oauth2.Token{})
}

func NewServer(addr string, store Store) *Server {
	return &Server{
		addr:            addr,
		store:           store,
		router:          httprouter.New(),
		Logger:          log.New(os.Stderr, "[Tabloid] ", log.LstdFlags),
		done:            make(chan struct{}),
		idleConnsClosed: make(chan struct{}),
	}
}

func (s *Server) loadConfig() error {
	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}

	if err = json.Unmarshal(b, &s.config); err != nil {
		return err
	}

	return nil
}

func (s *Server) Prepare() error {
	// github authentication
	err := s.loadConfig()
	if err != nil {
		return err
	}

	s.sessionStore = sessions.NewCookieStore([]byte(s.config.ServerSecret))

	s.oauthConfig = &oauth2.Config{
		ClientID:     s.config.ClientID,
		ClientSecret: s.config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: "",
		Scopes:      []string{"email"},
	}

	// database
	err = s.store.Connect()
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
			s.Logger.Fatal(err)
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
		b := make([]byte, 16)
		rand.Read(b)

		state := base64.URLEncoding.EncodeToString(b)

		session, _ := s.sessionStore.Get(req, sessionKey)
		session.Values["state"] = state
		session.Save(req, res)

		url := s.oauthConfig.AuthCodeURL(state)
		http.Redirect(res, req, url, 302)
	}
}

func (s *Server) HandleOAuthCallback() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		session, err := s.sessionStore.Get(req, sessionKey)
		if err != nil {
			http.Error(res, "Session aborted", http.StatusInternalServerError)
			return
		}

		if req.URL.Query().Get("state") != session.Values["state"] {
			http.Error(res, "no state match; possible csrf OR cookies not enabled", http.StatusInternalServerError)
			return
		}

		token, err := s.oauthConfig.Exchange(oauth2.NoContext, req.URL.Query().Get("code"))
		if err != nil {
			http.Error(res, "there was an issue getting your token", http.StatusInternalServerError)
			return
		}

		if !token.Valid() {
			http.Error(res, "retreived invalid token", http.StatusBadRequest)
			return
		}

		client := github.NewClient(s.oauthConfig.Client(oauth2.NoContext, token))

		user, _, err := client.Users.Get(context.Background(), "")
		if err != nil {
			http.Error(res, "error getting name", http.StatusInternalServerError)
			return
		}

		session.Values["githubUserName"] = user.Name
		session.Values["githubAccessToken"] = token
		err = session.Save(req, res)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "could not save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(res, req, "/", 302)
	}
}

func (s *Server) HandleOAuthDestroy() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		session, err := s.sessionStore.Get(req, sessionKey)
		if err != nil {
			http.Error(res, "aborted", http.StatusInternalServerError)
			return
		}

		// TODO put a sensible value in there
		session.Options.MaxAge = -1

		session.Save(req, res)
		http.Redirect(res, req, "/", 302)
	}
}

func (s *Server) getGithubStuff(session *sessions.Session) (map[string]interface{}, error) {
	renderData := map[string]interface{}{}
	if accessToken, ok := session.Values["githubAccessToken"].(*oauth2.Token); ok {
		client := github.NewClient(s.oauthConfig.Client(oauth2.NoContext, accessToken))

		user, _, err := client.Users.Get(context.Background(), "")
		if err != nil {
			return nil, err
		}

		renderData["User"] = user
		fmt.Println(renderData)

		var userMap map[string]interface{}
		mapstructure.Decode(user, &userMap)
		renderData["UserMap"] = userMap
	}

	return renderData, nil
}

func (s *Server) HandleIndex() httprouter.Handle {
	tmpl, err := template.ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		session, err := s.sessionStore.Get(req, sessionKey)
		if err != nil {
			http.Error(res, "Session aborted", http.StatusInternalServerError)
			return
		}

		data, err := s.getGithubStuff(session)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Couldn't fetch Github data", http.StatusMethodNotAllowed)
			return
		}

		res.Header().Set("Content-Type", "text/html")

		if req.Method != "GET" {
			http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
			return
		}

		stories, err := s.store.ListStories()
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Failed to list stories", http.StatusInternalServerError)
			return
		}

		vars := map[string]interface{}{
			"Stories": stories,
			"Session": data,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) HandleSubmit() httprouter.Handle {
	tmpl, err := template.ParseFiles("assets/templates/submit.html", "assets/templates/_header.html", "assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		session, err := s.sessionStore.Get(req, sessionKey)
		if err != nil {
			http.Error(res, "Session aborted", http.StatusInternalServerError)
			return
		}

		data, err := s.getGithubStuff(session)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Couldn't fetch Github data", http.StatusMethodNotAllowed)
			return
		}

		vars := map[string]interface{}{
			"Session": data,
		}

		err = tmpl.Execute(res, vars)
		if err != nil {
			s.Logger.Println(err)
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
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		session, err := s.sessionStore.Get(req, sessionKey)
		if err != nil {
			http.Error(res, "Session aborted", http.StatusInternalServerError)
			return
		}

		data, err := s.getGithubStuff(session)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Couldn't fetch Github data", http.StatusMethodNotAllowed)
			return
		}

		story, err := s.store.FindStory(params.ByName("id"))
		if err != nil {
			s.Logger.Println(err)
			// TODO deal with 404
			http.Error(res, "Failed to find story", http.StatusInternalServerError)
		}

		comments, err := s.store.ListComments(strconv.Itoa(int(story.ID)))
		if err != nil {
			s.Logger.Println(err)
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
			s.Logger.Println(err)
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) HandleSubmitAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")

		err := req.ParseForm()
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Couldn't parse form", http.StatusBadRequest)
		}

		s.Logger.Println(req.Form)

		title := req.Form["title"][0]
		body := strings.TrimSpace(req.Form["body"][0])
		url := req.Form["url"][0]

		story := &Story{
			Author: "Thomas",
			Title:  title,
			Body:   body,
			URL:    url,
		}

		err = s.store.InsertStory(story)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Cannot insert item", http.StatusMethodNotAllowed)
			return
		}

		http.Redirect(res, req, "/", http.StatusFound)
	}
}

func (s *Server) HandleSubmitCommentAction() httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
		res.Header().Set("Content-Type", "text/html")
		story, err := s.store.FindStory(params.ByName("id"))
		if err != nil {
			s.Logger.Println(err)
			// TODO deal with 404
			http.Error(res, "Failed to find story", http.StatusInternalServerError)
			return
		}

		err = req.ParseForm()
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Couldn't parse form", http.StatusBadRequest)
		}

		var comment *Comment
		body := strings.TrimSpace(req.Form["body"][0])
		_parentCommentID := req.Form["parent-id"][0]

		// if top-level comment
		if _parentCommentID == "" {
			comment = &Comment{
				Body:    body,
				StoryID: story.ID,
			}
		} else {
			parentCommentID, err := strconv.Atoi(_parentCommentID)
			if err != nil {
				s.Logger.Println(err)
				http.Error(res, "Cannot parse parent ID", http.StatusUnprocessableEntity)
				return
			}

			comment = &Comment{
				Body:    body,
				StoryID: story.ID,
				ParentCommentID: sql.NullInt64{
					Int64: int64(parentCommentID),
					Valid: true,
				},
			}
		}

		err = s.store.InsertComment(comment)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Cannot insert item", http.StatusMethodNotAllowed)
			return
		}

		storyPath := fmt.Sprintf("/stories/%v/comments", story.ID)
		http.Redirect(res, req, storyPath, http.StatusFound)
	}
}
