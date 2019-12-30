package github_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/oauth2"
)

const (
	sessionKey = "tabloid-session"
)

type Handler struct {
	userData     map[string]interface{}
	sessionStore *sessions.CookieStore
	clientID     string
	clientSecret string
	logger       *log.Logger
	oauthConfig  *oauth2.Config
}

func New(serverSecret string, clientID string, clientSecret string) *Handler {
	sessionStore := sessions.NewCookieStore([]byte(serverSecret))
	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: "",
		Scopes:      []string{"email"},
	}
	return &Handler{
		sessionStore: sessionStore,
		oauthConfig:  oauthConfig,
	}
}

func (h *Handler) LoadUserData(req *http.Request) error {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return err
	}

	if accessToken, ok := session.Values["githubAccessToken"].(*oauth2.Token); ok {
		h.userData = map[string]interface{}{}
		client := github.NewClient(h.oauthConfig.Client(oauth2.NoContext, accessToken))

		user, _, err := client.Users.Get(context.Background(), "")
		if err != nil {
			return err
		}

		h.userData["User"] = user

		var userMap map[string]interface{}
		mapstructure.Decode(user, &userMap)
		h.userData["UserMap"] = userMap
		return nil
	}

	return nil
}

func (h *Handler) CurrentUser(req *http.Request) (map[string]interface{}, error) {
	return h.userData, nil
}

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) {
	b := make([]byte, 16)
	rand.Read(b)

	state := base64.URLEncoding.EncodeToString(b)

	session, _ := h.sessionStore.Get(req, sessionKey)
	session.Values["state"] = state
	session.Save(req, res)

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(res, req, url, 302)
}

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		http.Error(res, "Session aborted", http.StatusInternalServerError)
		return
	}

	if req.URL.Query().Get("state") != session.Values["state"] {
		http.Error(res, "no state match; possible csrf OR cookies not enabled", http.StatusInternalServerError)
		return
	}

	token, err := h.oauthConfig.Exchange(oauth2.NoContext, req.URL.Query().Get("code"))
	if err != nil {
		http.Error(res, "there was an issue getting your token", http.StatusInternalServerError)
		return
	}

	if !token.Valid() {
		http.Error(res, "retrieved invalid token", http.StatusBadRequest)
		return
	}

	client := github.NewClient(h.oauthConfig.Client(oauth2.NoContext, token))

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		http.Error(res, "error getting name", http.StatusInternalServerError)
		return
	}

	session.Values["githubUserName"] = user.Name
	session.Values["githubAccessToken"] = token
	err = session.Save(req, res)
	if err != nil {
		h.logger.Println(err)
		http.Error(res, "could not save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/", 302)
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		http.Error(res, "aborted", http.StatusInternalServerError)
		return
	}

	// kill the session
	session.Options.MaxAge = -1

	session.Save(req, res)
	http.Redirect(res, req, "/", 302)
}
