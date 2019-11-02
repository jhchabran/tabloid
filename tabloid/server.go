package tabloid

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type indexHandler struct {
	template *template.Template
	server   *Server
}

type Server struct {
	Logger *log.Logger
	addr   string
	db     *sqlx.DB
	mux    *http.ServeMux
}

func NewServer(addr string) *Server {
	return &Server{
		addr:   addr,
		mux:    http.NewServeMux(),
		Logger: log.New(os.Stderr, "[Tabloid] ", log.LstdFlags),
	}
}

func (s *Server) Start() error {
	err := s.connectToDatabase()
	if err != nil {
		return err
	}

	s.mux.HandleFunc("/", s.HandleIndex())
	s.mux.HandleFunc("/submit", s.HandleSubmit())
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./assets/static")))
	s.mux.Handle("/static/", staticHandler)

	http.ListenAndServe(s.addr, s)

	// we'll never get here
	return nil
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(res, req)
}

func (s *Server) connectToDatabase() error {
	db, err := sqlx.Connect("postgres", "user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	if err != nil {
		return err
	}

	s.db = db

	return nil
}

func (s *Server) InsertStory(item *Story) error {
	_, err := s.db.Exec("INSERT INTO stories (title, url, body, score, author, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		item.Title, item.URL, item.Body, item.Score, item.Author, time.Now(),
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *Server) ListStories() ([]*Story, error) {
	stories := []*Story{}
	err := s.db.Select(&stories, "SELECT * FROM stories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}

	return stories, nil
}

func (s *Server) HandleIndex() http.HandlerFunc {
	tmpl, err := template.ParseFiles("assets/templates/index.html",
		"assets/templates/_header.html",
		"assets/templates/_footer.html",
		"assets/templates/_story.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/html")

		if req.Method != "GET" {
			http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
			return
		}

		stories, err := s.ListStories()
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

func (s *Server) HandleSubmit() http.HandlerFunc {
	tmpl, err := template.ParseFiles("assets/templates/submit.html", "assets/templates/_header.html", "assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/html")

		if req.Method != "GET" && req.Method != "POST" {
			http.Error(res, "Only GET or POST is allowed", http.StatusMethodNotAllowed)
			return
		}

		if req.Method == "GET" {
			err = tmpl.Execute(res, nil)
			if err != nil {
				s.Logger.Println(err)
				http.Error(res, "Failed to render template", http.StatusInternalServerError)
				return
			}

		} else {
			err := req.ParseForm()
			if err != nil {
				s.Logger.Println(err)
				http.Error(res, "Couldn't parse form", http.StatusBadRequest)
			}

			s.Logger.Println(req.Form)

			title := req.Form["title"][0]
			body := req.Form["body"][0]
			url := req.Form["url"][0]

			item := &Story{
				Author: "Thomas",
				Title:  title,
				Body:   body,
				URL:    url,
			}

			err = s.InsertStory(item)
			if err != nil {
				s.Logger.Println(err)
				http.Error(res, "Cannot insert item", http.StatusMethodNotAllowed)
				return
			}

			http.Redirect(res, req, "/", http.StatusFound)
		}
	}
}
