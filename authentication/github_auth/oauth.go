package github_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/authentication"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
)

const (
	sessionKey = "tabloid-session"
)

// TODO wrap in a config struct?
type Handler struct {
	// useless atm, but keeping around to dig around whatever I could need
	// at this stage.
	sessionStore *sessions.CookieStore
	logger       zerolog.Logger
	oauthConfig  *oauth2.Config
}

func New(serverSecret string, clientID string, clientSecret string, logger zerolog.Logger) *Handler {
	sessionStore := sessions.NewCookieStore([]byte(serverSecret))
	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: "",
		Scopes:      []string{"user:email"},
	}
	return &Handler{
		sessionStore: sessionStore,
		oauthConfig:  oauthConfig,
		logger:       logger,
	}
}

// TODO comment that properly
// side effect: load into session
// returns what was stored in the session
func (h *Handler) LoadUserData(accessToken *oauth2.Token, req *http.Request, res http.ResponseWriter) (*authentication.User, error) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(h.oauthConfig.Client(context.Background(), accessToken))

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, err
	}

	userSession := &authentication.User{
		Login:     *user.Login,
		AvatarURL: *user.AvatarURL,
	}

	// email can be defined as private in Github, and if that's the case
	// this field will be nil.
	if user.Email != nil {
		userSession.Email = *user.Email
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
		return nil, nil
	}

	var userSession authentication.User
	err = json.Unmarshal(b, &userSession)
	if err != nil {
		return nil, err
	}

	return &userSession, nil
}

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) error {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return err
	}

	state := base64.URLEncoding.EncodeToString(b)

	session, _ := h.sessionStore.Get(req, sessionKey)
	session.Values["state"] = state
	err = session.Save(req, res)
	if err != nil {
		return err
	}

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(res, req, url, 302)
	return nil
}

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request, beforeWriteCallback func(*authentication.User) error) error {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return err
	}

	if req.URL.Query().Get("state") != session.Values["state"] {
		return fmt.Errorf("no state match; possible csrf OR cookies not enabled")
	}

	token, err := h.oauthConfig.Exchange(context.Background(), req.URL.Query().Get("code"))
	if err != nil {
		return err
	}

	if !token.Valid() {
		return tabloid.BadRequest(fmt.Errorf("retrieve invalid token"))
	}

	u, err := h.LoadUserData(token, req, res)
	if err != nil {
		return err
	}

	err = beforeWriteCallback(u)
	if err != nil {
		return err
	}

	http.Redirect(res, req, "/", 302)
	return nil
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) error {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		return err
	}

	// kill the session
	session.Options.MaxAge = -1
	session.Values["user"] = nil // TODO max age probably makes this unnecessary
	err = session.Save(req, res)
	if err != nil {
		return err
	}

	http.Redirect(res, req, "/", 302)
	return nil
}
