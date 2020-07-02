package fake_auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid/authentication"
	"golang.org/x/oauth2"
)

const sessionKey = "fake_auth_key"

type Handler struct {
	userData     map[string]interface{}
	user         *authentication.User
	sessionStore *sessions.CookieStore
	serverUrl    string
	counter      int // used to return a different user for each auth
}

func New(sessionStore *sessions.CookieStore) *Handler {
	return &Handler{
		sessionStore: sessionStore,
	}
}

func (h *Handler) SetServerURL(url string) {
	h.serverUrl = url
}

func (h *Handler) LoadUserData(accessToken *oauth2.Token, req *http.Request, res http.ResponseWriter) (*authentication.User, error) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return nil, err
	} // TODO do I need this?

	userSession := &authentication.User{
		Login:     "fakeLogin" + strconv.Itoa(h.counter),
		AvatarURL: "https://www.placecage.com/g/200/200",
	}
	b, err := json.Marshal(userSession)
	if err != nil {
		return nil, err
	}

	session.Values["user"] = b
	if err := session.Save(req, res); err != nil {
		return nil, err
	}

	return userSession, nil
}

func (h *Handler) CurrentUser(req *http.Request) (*authentication.User, error) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return nil, err
	}

	var b []byte
	b, ok := session.Values["user"].([]byte)
	if !ok {
		log.Println("no session")
		return nil, nil
	}

	var userSession authentication.User
	err = json.Unmarshal(b, &userSession)
	if err != nil {
		return nil, err
	}

	return &userSession, nil
}

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, sessionKey)
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

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request, beforeWriteCallback func(*authentication.User) error) {
	u, err := h.LoadUserData(nil, req, res)
	if err != nil {
		http.Error(res, "couldn't load user data from fake auth", 500)
		return
	}

	err = beforeWriteCallback(u)
	if err != nil {
		http.Error(res, "failed to execute oauth callback", 500)
		return
	}

	http.Redirect(res, req, "/", 302)
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) {
	// TODO error
	session, _ := h.sessionStore.Get(req, sessionKey)
	session.Options.MaxAge = -1
	session.Save(req, res)

	http.Redirect(res, req, "/", 302)
}
