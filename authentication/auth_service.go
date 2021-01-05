package authentication

import (
	"net/http"

	"golang.org/x/oauth2"
)

// An OAuthHandler is responsible of providing the callbacks to interact
// with an OAuth provider.
type OAuthHandler interface {
	Start(res http.ResponseWriter, req *http.Request)
	Callback(res http.ResponseWriter, req *http.Request, beforeWriteCallback func(*User) error)
	Destroy(res http.ResponseWriter, req *http.Request)
}

// An AuthService wraps OAuth and a access to the current user.
type AuthService interface {
	OAuthHandler
	CurrentUser(req *http.Request) (*User, error)
	LoadUserData(token *oauth2.Token, req *http.Request, res http.ResponseWriter) (*User, error)
}

// A User is a convenient structure to hold user data coming from Github.
type User struct {
	AvatarURL string
	Login     string
	Email     string
	// No reason to store the token for now
	// AccessToken *oauth2.Token
}
