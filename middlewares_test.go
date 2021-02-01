package tabloid

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/julienschmidt/httprouter"
)

func TestWithMiddlewares(t *testing.T) {
	c := qt.New(t)

	handler := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error { return nil }

	c.Run("calls middlewares", func(c *qt.C) {
		s1 := false
		m1 := func(h HandleE) HandleE { s1 = true; return h }

		withMiddlewares(func(m middleware) { m(handler) }, m1)
		c.Assert(s1, qt.IsTrue)
	})

	c.Run("passing m1, m2, m3 run them in that order", func(c *qt.C) {
		trace := []int{}
		m1 := func(h HandleE) HandleE {
			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
				trace = append(trace, 1)
				return h(w, r, p)
			}
		}
		m2 := func(h HandleE) HandleE {
			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
				trace = append(trace, 2)
				return h(w, r, p)
			}
		}
		m3 := func(h HandleE) HandleE {
			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
				trace = append(trace, 3)
				return h(w, r, p)
			}
		}

		var h HandleE
		withMiddlewares(func(m middleware) { h = m(handler) },
			m1,
			m2,
			m3)

		_ = h(httptest.NewRecorder(), &http.Request{}, httprouter.Params{})

		c.Assert(trace, qt.DeepEquals, []int{1, 2, 3})
	})
}

// TODO
// func TestLoadSessionMiddleware(t *testing.T) {
// }
