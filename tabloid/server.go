package tabloid

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
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

func shiftPath(p string) (head, tail string) {
	p = path.Clean("/" + p)
	i := strings.Index(p[1:], "/") + 1
	if i <= 0 {
		return p[1:], "/"
	}
	return p[1:i], p[i:]
}

func (s *Server) HandleIndex() http.HandlerFunc {
	b, err := ioutil.ReadFile("assets/html/index.html")
	if err != nil {
		s.Logger.Fatal(err)
	}

	tmpl, err := template.New("index").Parse(string(b))
	if err != nil {
		s.Logger.Fatal(err)
	}

	return func(res http.ResponseWriter, req *http.Request) {
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
			http.Error(res, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}
