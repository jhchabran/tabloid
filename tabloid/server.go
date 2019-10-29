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

func (s *Server) InsertItem(item *Item) error {
	_, err := s.db.Exec("INSERT INTO items (title, body, score, author, created_at) VALUES ($1, $2, $3, $4, $5)",
		item.Title, item.Body, item.Score, item.Author, time.Now(),
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *Server) ListItems() ([]*Item, error) {
	items := []*Item{}
	err := s.db.Select(&items, "SELECT * FROM items ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (s *Server) HandleIndex() http.HandlerFunc {
	tmpl, err := template.ParseFiles("assets/templates/index.html", "assets/templates/_header.html", "assets/templates/_footer.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/html")

		if req.Method != "GET" {
			http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
			return
		}

		items, err := s.ListItems()
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Failed to list items", http.StatusInternalServerError)
			return
		}

		vars := map[string]interface{}{
			"text":  "foobar",
			"items": items,
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

		if req.Method != "GET" || req.Method != "POST" {
			http.Error(res, "Only GET or POST is allowed", http.StatusMethodNotAllowed)
			return
		}

		// TODO handle post

		err = tmpl.Execute(res, nil)
		if err != nil {
			s.Logger.Println(err)
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}
