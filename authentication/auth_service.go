package authentication

import "net/http"

type OAuthHandler interface {
	Start(res http.ResponseWriter, req *http.Request)
	Callback(res http.ResponseWriter, req *http.Request)
	Destroy(res http.ResponseWriter, req *http.Request)
}

type AuthService interface {
	OAuthHandler
	CurrentUser(req *http.Request) (map[string]interface{}, error)
	LoadUserData(req *http.Request) error
}
