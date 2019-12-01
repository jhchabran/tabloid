package tabloid

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	_ "github.com/lib/pq"
)

type Server struct {
	Logger          *log.Logger
	addr            string
	store           Store
	dbString        string
	router          *httprouter.Router
	done            chan struct{}
	idleConnsClosed chan struct{}
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

func (s *Server) Prepare() error {
	err := s.store.Connect()
	if err != nil {
		return err
	}

	s.router.GET("/", s.HandleIndex())
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

func (s *Server) HandleIndex() httprouter.Handle {
	dir, err := os.Getwd()
	if err != nil {
		s.Logger.Fatal(err)
	}
	s.Logger.Println(dir)

	tmpl, err := template.ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
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
			"stories": stories,
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

		err = tmpl.Execute(res, nil)
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
