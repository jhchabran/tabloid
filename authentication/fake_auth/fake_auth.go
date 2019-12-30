package fake_auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type Handler struct {
	userData     map[string]interface{}
	sessionStore *sessions.CookieStore
	serverUrl    string
}

func New(sessionStore *sessions.CookieStore) *Handler {
	return &Handler{
		sessionStore: sessionStore,
	}
}

func (h *Handler) SetServerURL(url string) {
	h.serverUrl = url
}

func (h *Handler) LoadUserData(req *http.Request) error {
	return nil
}

func (h *Handler) CurrentUser(req *http.Request) (map[string]interface{}, error) {
	return h.userData, nil
}

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) {
	http.Redirect(res, req, h.serverUrl+"/oauth/authorize", 302)
}

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, "fake auth key")
	if err != nil {
		http.Error(res, "session aborted", http.StatusInternalServerError)
	}

	session.Values["githubUserName"] = "jhchabran"
	session.Values["githubAccessToken"] = "test-token"

	err = session.Save(req, res)
	if err != nil {
		http.Error(res, "couldn't save session", http.StatusInternalServerError)
	}

	http.Redirect(res, req, "/", 302)
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) {
	// TODO error
	session, _ := h.sessionStore.Get(req, "fake auth key")
	session.Options.MaxAge = -1
	session.Save(req, res)

	http.Redirect(res, req, "/", 302)
}
