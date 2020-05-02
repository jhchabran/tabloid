package authentication

import "net/http"

// An OAuthHandler is responsible of providing the callbacks to interact
// with an OAuth provider.
type OAuthHandler interface {
	Start(res http.ResponseWriter, req *http.Request)
	Callback(res http.ResponseWriter, req *http.Request)
	Destroy(res http.ResponseWriter, req *http.Request)
}

// An AuthService wraps OAuth and a access to the current user.
type AuthService interface {
	OAuthHandler
	CurrentUser(req *http.Request) (*User, error)
	LoadUserData(req *http.Request) error
}

// A User is a convenient structure to hold user data coming from Github.
type User struct {
	AvatarURL string
	Login     string
}
