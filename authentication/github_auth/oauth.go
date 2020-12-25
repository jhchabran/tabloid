package github_auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
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

func (h *Handler) Start(res http.ResponseWriter, req *http.Request) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		http.Error(res, "", http.StatusInternalServerError)
		return
	}

	state := base64.URLEncoding.EncodeToString(b)

	session, _ := h.sessionStore.Get(req, sessionKey)
	session.Values["state"] = state
	err = session.Save(req, res)
	if err != nil {
		http.Error(res, "can't save session", http.StatusInternalServerError)
		return
	}

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(res, req, url, 302)
}

func (h *Handler) Callback(res http.ResponseWriter, req *http.Request, beforeWriteCallback func(*authentication.User) error) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		http.Error(res, "Session aborted", http.StatusInternalServerError)
		return
	}

	if req.URL.Query().Get("state") != session.Values["state"] {
		http.Error(res, "no state match; possible csrf OR cookies not enabled", http.StatusInternalServerError)
		return
	}

	token, err := h.oauthConfig.Exchange(context.Background(), req.URL.Query().Get("code"))
	if err != nil {
		http.Error(res, "there was an issue getting your token", http.StatusInternalServerError)
		return
	}

	if !token.Valid() {
		http.Error(res, "retrieved invalid token", http.StatusBadRequest)
		return
	}

	u, err := h.LoadUserData(token, req, res)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to load user data from Github")
		http.Error(res, "failed to load user data from Github", 500)
		return
	}

	err = beforeWriteCallback(u)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to execute oauth callback")
		http.Error(res, "failed to execute oauth callback", 500)
		return
	}

	http.Redirect(res, req, "/", 302)
}

func (h *Handler) Destroy(res http.ResponseWriter, req *http.Request) {
	session, err := h.sessionStore.Get(req, sessionKey)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to destroy session")
		http.Error(res, "failed to destroy session", http.StatusInternalServerError)
		return
	}

	// kill the session
	session.Options.MaxAge = -1
	session.Values["user"] = nil // TODO max age probably makes this unnecessary
	err = session.Save(req, res)
	if err != nil {
		http.Error(res, "can't save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/", 302)
}
