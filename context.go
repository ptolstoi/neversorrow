package neversorrow

import (
	"encoding/json"
	"net/http"

	"github.com/ptolstoi/neversorrow/errors"
)

type ContextKey string

const RequestContextKey ContextKey = "requestContextKey"

type Context interface {
	Error(error)
	ResponseWithJSON(interface{})

	App() App
	ResponseWriter() http.ResponseWriter
	Request() *http.Request

	Params() map[string]string
}

type requestContext struct {
	app App
	w   http.ResponseWriter
	req *http.Request

	params map[string]string
}

func newContext(app App, w http.ResponseWriter, req *http.Request) Context {
	return &requestContext{
		app: app,
		w:   w,
		req: req,

		params: make(map[string]string, 0),
	}
}

func (ctx *requestContext) App() App {
	return ctx.app
}

func (ctx *requestContext) ResponseWriter() http.ResponseWriter {
	return ctx.w
}

func (ctx *requestContext) Request() *http.Request {
	return ctx.req
}

func (ctx *requestContext) Params() map[string]string {
	return ctx.params
}

func (ctx *requestContext) Error(err error) {
	statusCode := http.StatusInternalServerError
	var stacktrace []string

	switch err.(type) {
	case errors.Error:
		err := err.(errors.Error)
		statusCode = err.StatusCode()
		if ctx.app.Config().ShowStacktrace == "" {
			stacktrace = err.Stacktrace()
		}
	}

	ctx.w.WriteHeader(statusCode)
	ctx.w.Header().Add("content-type", "application/json")
	json.NewEncoder(ctx.w).Encode(struct {
		Error      string   `json:"error"`
		Message    string   `json:"message"`
		Stacktrace []string `json:"stacktrace,omitempty"`
	}{
		Error:      "error",
		Message:    err.Error(),
		Stacktrace: stacktrace,
	})
}

func (ctx *requestContext) ResponseWithJSON(v interface{}) {
	ctx.w.Header().Add("content-type", "application/json")
	json.NewEncoder(ctx.w).Encode(v)
}
