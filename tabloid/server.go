package tabloid

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type indexHandler struct {
	template *template.Template
}

type Server struct {
	Logger *log.Logger
	addr   string
	db     *sqlx.DB
	wg     sync.WaitGroup

	templates struct {
		Index *template.Template
	}

	handlers struct {
		index *indexHandler
	}
}

func NewServer(addr string) *Server {
	return &Server{
		addr:   addr,
		Logger: log.New(os.Stderr, "[Tabloid] ", log.LstdFlags),
	}
}

func (s *Server) Start() error {
	err := s.connectToDatabase()
	if err != nil {
		return err
	}

	err = s.loadTemplates()
	if err != nil {
		return err
	}

	s.handlers.index = &indexHandler{
		template: s.templates.Index,
	}

	http.ListenAndServe(s.addr, s)

	// we'll never get here
	return nil
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var head string
	head, req.URL.Path = shiftPath(req.URL.Path)

	if head == "" {
		s.handlers.index.ServeHTTP(res, req)
		return
	}
}

func (s *Server) connectToDatabase() error {
	db, err := sqlx.Connect("postgres", "user=postgres dbname=tabloid sslmode=disable password=postgres host=127.0.0.1")
	if err != nil {
		return err
	}

	s.db = db

	return nil
}

func (s *Server) loadTemplates() error {
	b, err := ioutil.ReadFile("assets/html/index.html")
	if err != nil {
		return err
	}

	t, err := template.New("index").Parse(string(b))
	if err != nil {
		return err
	}

	s.templates.Index = t

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

func shiftPath(p string) (head, tail string) {
	p = path.Clean("/" + p)
	i := strings.Index(p[1:], "/") + 1
	if i <= 0 {
		return p[1:], "/"
	}
	return p[1:i], p[i:]
}

func (h *indexHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(res, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := map[string]interface{}{
		"text":  "foobar",
		"items": []string{"title A", "title B"},
	}

	err := h.template.Execute(res, vars)
	if err != nil {
		http.Error(res, "Failed to render template", http.StatusInternalServerError)
		return
	}
}
