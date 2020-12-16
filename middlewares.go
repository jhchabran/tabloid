package tabloid

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/jhchabran/tabloid/authentication"
	"github.com/julienschmidt/httprouter"
)

// middleware is a convenient type for declaring middlewares.
type middleware func(httprouter.Handle) httprouter.Handle

// httpMiddleware is a convenient type for declaring middlewares.
type httpMiddleware func(http.Handler) http.Handler

// contextKey is a type for storing values in each request context.
type contextKey string

// String returns a stringified context key.
func (k contextKey) String() string { return string(k) }

// ctxKeySession is the context key for storing the current user session in a context
var ctxKeySession = contextKey("session")

// ctxKeyUser is the context key for storing the current user record in a context
var ctxKeyUser = contextKey("user")

// ctxSession is a helper func to fetch the user session from the context.
func ctxSession(ctx context.Context) *authentication.User {
	v := ctx.Value(ctxKeySession)
	if v != nil {
		return ctx.Value(ctxKeySession).(*authentication.User)
	} else {
		return nil
	}
}

// ctxUser is a helper func to fetch the user session from the context.
func ctxUser(ctx context.Context) *User {
	v := ctx.Value(ctxKeyUser)
	if v != nil {
		return ctx.Value(ctxKeyUser).(*User)
	} else {
		return nil
	}
}

// withMiddlewares is a helper function to declare routes with middlewares more easily.
// The caller declares its routes in the body on the f function, calling f's argument on its
// httprouter.Handle to wrap them.
func withMiddlewares(f func(middleware), middlewares ...middleware) {
	wrapper := func(handle httprouter.Handle) httprouter.Handle {
		h := handle
		for i := len(middlewares) - 1; i >= 0; i-- {
			m := middlewares[i]
			h = m(h)
		}
		return h
	}

	f(wrapper)
}

// loadSessionMiddleware fetches the user session data through the AuthService
// and stores it in the request context. If there's no session it will assign nil in
// the context to thes session key.
func (s *Server) loadSessionMiddleware() middleware {
	return func(next httprouter.Handle) httprouter.Handle {
		return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			userData, err := s.authService.CurrentUser(r)
			if err != nil {
				s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
				http.Error(w, "Failed to fetch session data", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeySession, userData)
			next(w, r.WithContext(ctx), p)
		})
	}
}

// loadUser fetches the user from the database and stores it in the request context. If there's an error
// it will interrupt the middlware chain, returning an http error.
//
// If there is no session, it will send a http error about not being authorized.
func (s *Server) loadUserMiddleware() middleware {
	return func(next httprouter.Handle) httprouter.Handle {
		return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			s.Logger.Debug().Msg("user")
			session := ctxSession(r.Context())

			if session == nil {
				s.Logger.Debug().Msg("Attempt to load user with no session, halting middleware chain and redirecting.")
				// TODO some flash notice would be useful here.
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userRecord, err := s.store.FindUserByLogin(session.Login)
			if err != nil {
				s.Logger.Error().Err(err).Msg("Failed to fetch user from db")
				http.Error(w, "Failed to fetch user from database", http.StatusInternalServerError)
				return
			}

			// there is a session but no user in the database, wiping the session
			if userRecord == nil {
				s.authService.Destroy(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUser, userRecord)
			next(w, r.WithContext(ctx), p)
		})
	}
}

func (s *Server) httpVerbFormUnwrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// if we have a POST method, we need to read the body to get to the form field name "_method", in order to
		// swap the http method for the request. Because req.ParseForm() consumes the body, we have to save it
		// and pass new one to the next middleware.
		if req.Method == http.MethodPost {
			// save the original body and let's make sure we don't blow up on a huge body (do what req.ParseForm()
			// is doing basically).
			maxFormSize := int64(10 << 20) // 10 MB is a lot of text.
			reader := io.LimitReader(req.Body, maxFormSize+1)
			body, err := ioutil.ReadAll(reader)
			defer req.Body.Close()

			if err != nil {
				s.Logger.Error().Err(err).Msg("can't read request body")
				http.Error(res, "", http.StatusBadRequest)
				return
			}

			if int64(len(body)) > maxFormSize {
				s.Logger.Warn().Msg("http: POST too large")
				http.Error(res, "http: POST too large", http.StatusBadRequest)
				return
			}

			// req.ParseForm() consumes req.Body, so we need to give it one.
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

			err = req.ParseForm()
			if err != nil {
				http.Error(res, "", http.StatusBadRequest)
			}
			method := req.Form.Get("_method")

			switch strings.ToUpper(method) {
			case "PUT":
				req.Method = http.MethodPut
			case "PATCH":
				req.Method = http.MethodPatch
			case "DELETE":
				req.Method = http.MethodPatch
			case "POST":
			case "":
			default:
				http.Error(res, "", http.StatusBadRequest)
			}

			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}

		next.ServeHTTP(res, req)
	})
}
