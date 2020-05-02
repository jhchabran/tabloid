package fake_auth

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid/authentication"
)

const cookieKey = "fake_auth_key"

type Handler struct {
	userData     map[string]interface{}
	user         *authentication.User
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

func (h *Handler) CurrentUser(req *http.Request) (*authentication.User, error) {
	return h.user, nil
}

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, cookieKey)
	if err != nil {
		panic(err)
	}

	session.Values["state"] = "state"
	err = session.Save(req, res)
	if err != nil {
		http.Error(res, "cannot save cookies", 500)
		return
	}

	http.Redirect(res, req, h.serverUrl+"/oauth/authorize", 302)
}

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, cookieKey)
	if err != nil {
		http.Error(res, "session aborted", http.StatusInternalServerError)
		return
	}

	// TODO do I need this?
	session.Values["githubUserName"] = "jhchabran"
	session.Values["githubAccessToken"] = "test-token"

	h.user = &authentication.User{
		Login:     "fakeLogin",
		AvatarURL: "https://www.placecage.com/g/200/200",
	}

	err = session.Save(req, res)
	if err != nil {
		http.Error(res, "couldn't save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/", 302)
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) {
	// TODO error
	session, _ := h.sessionStore.Get(req, cookieKey)
	session.Options.MaxAge = -1
	session.Save(req, res)

	http.Redirect(res, req, "/", 302)
}
