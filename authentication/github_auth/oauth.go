package github_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid/authentication"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/oauth2"
)

const (
	sessionKey = "tabloid-session"
)

type Handler struct {
	// useless atm, but keeping around to dig around whatever I could need
	// at this stage.
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

// TODO comment that properly
// side effect: load into session
// returns what was stored in the session
func (h *Handler) LoadUserData(req *http.Request, res http.ResponseWriter) (*authentication.User, error) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return nil, err
	}

	accessToken, ok := session.Values["githubAccessToken"].(*oauth2.Token)
	if !ok {
		return nil, fmt.Errorf("inconsistent state")
	}

	userData := map[string]interface{}{}
	client := github.NewClient(h.oauthConfig.Client(oauth2.NoContext, accessToken))

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, err
	}

	userSession := &authentication.User{
		Login:     *user.Login,
		AvatarURL: *user.AvatarURL,
	}

	userData["User"] = user

	var userMap map[string]interface{}
	mapstructure.Decode(user, &userMap)
	userData["UserMap"] = userMap

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

	// client := github.NewClient(h.oauthConfig.Client(oauth2.NoContext, token))

	// user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		http.Error(res, "error getting name", http.StatusInternalServerError)
		return
	}

	// TODO it seems I don't need this at all.
	// session.Values["githubUserName"] = user.Name
	// session.Values["githubAccessToken"] = token
	// err = session.Save(req, res)
	if err != nil {
		h.logger.Println(err)
		http.Error(res, "could not save session", http.StatusInternalServerError)
		return
	}

	_, err = h.LoadUserData(req, res)
	if err != nil {
		http.Error(res, "couldn't load user data from Github", 500)
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
	session.Values["user"] = nil // TODO max age probably makes this unnecessary
	session.Save(req, res)

	http.Redirect(res, req, "/", 302)
}
