package neversorrow

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/ptolstoi/neversorrow/errors"
)

type OnHandler func(App)
type OnServeHTTPHandler func(Context)

type App interface {
	Config() Config

	Start() error
	Stop() error
	Close() error

	RunUntilSignal() App

	OnStart(OnHandler) App
	OnStop(OnHandler) App
	OnClose(OnHandler) App
	OnServeHTTP(OnServeHTTPHandler) App

	AddRoute(method, route string, handler OnServeHTTPHandler) App
}

type app struct {
	config Config

	http   *http.Server
	router *httprouter.Router

	eventHandler     map[string]OnHandler
	serveHTTPHandler OnServeHTTPHandler
}

func New(config Config) App {
	app := app{
		config: config,

		eventHandler: make(map[string]OnHandler),
		router:       httprouter.New(),
	}
	app.http = &http.Server{
		Handler: &app,
	}

	app.router.NotFound = app.NotFound()
	return &app
}

func (app *app) Config() Config {
	return app.config
}

func (app *app) Start() error {
	networkSocketType := "unix"
	if strings.Contains(app.config.Address, ":") {
		networkSocketType = "tcp"
	}

	if networkSocketType == "unix" {
		if err := syscall.Unlink(app.config.Address); err != nil && !os.IsNotExist(err) {
			log.Fatalf("error when unlinking %s: %s", app.config.Address, err.Error())
		}
	}

	listener, err := net.Listen(networkSocketType, app.config.Address)
	if err != nil {
		return err
	}

	go func() {
		prefix := ""
		if networkSocketType == "tcp" {
			prefix = "http://"
		}

		log.Printf("listening on %s%v\n", prefix, listener.Addr())
		app.http.Serve(listener)
	}()

	app.emit("start")

	return nil
}

func (app *app) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.http.Shutdown(ctx); err != nil {
		return err
	}

	app.emit("stop")

	return nil
}

func (app *app) Close() error {
	app.emit("close")

	return nil
}

func (app *app) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := newContext(app, w, req)
	req = req.WithContext(context.WithValue(req.Context(), RequestContextKey, ctx))
	if _, ok := ctx.(*requestContext); ok {
		// ctx.req = req
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered! %v", r)

			ctx.Error(errors.NewWithCode(fmt.Sprintf("Panic captured! %+v", r), 500))
		}
	}()

	log.Printf("[%s] url=%v", req.Method, req.URL.String())

	if req.Method == http.MethodGet {
		if req.URL.Path == "/version" {
			ctx.ResponseWithJSON(&struct {
				Version        string `json:"version"`
				BuildTime      string `json:"BuildTime"`
				ShowStacktrace bool   `json:"showStacktrace"`
			}{
				Version:        app.config.Version,
				BuildTime:      app.config.BuildTime,
				ShowStacktrace: app.config.ShowStacktrace == "",
			})
			return
		}
	}

	if handler := app.serveHTTPHandler; handler != nil {
		handler(ctx)
	} else {
		app.router.ServeHTTP(w, req)
	}
}

func (app *app) emit(name string) {
	if handler, ok := app.eventHandler[name]; ok {
		handler(app)
	}
}

func (app *app) on(name string, fn OnHandler) {
	app.eventHandler[name] = fn
}

func (app *app) OnStart(fn OnHandler) App {
	app.on("start", fn)
	return app
}

func (app *app) OnStop(fn OnHandler) App {
	app.on("stop", fn)
	return app
}

func (app *app) OnClose(fn OnHandler) App {
	app.on("close", fn)
	return app
}

func (app *app) OnServeHTTP(fn OnServeHTTPHandler) App {
	app.serveHTTPHandler = fn
	return app
}

func (app *app) RunUntilSignal() App {
	defer app.Close()

	app.Start()

	stopChannel := make(chan os.Signal, 1)
	signal.Notify(stopChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	<-stopChannel

	app.Stop()

	return app
}

func (app *app) AddRoute(method, route string, handler OnServeHTTPHandler) App {
	app.router.Handle(method, route, func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		var ctx Context
		if _ctx, ok := req.Context().Value(RequestContextKey).(Context); ok {
			ctx = _ctx
		}

		if ctx == nil {
			ctx = newContext(app, w, req)
		}

		params := ctx.Params()
		for _, p := range p {
			params[p.Key] = p.Value
		}

		handler(ctx)
	})

	return app
}

func (app *app) NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := app.contextFromRequest(w, req)
		ctx.Error(errors.NewWithCode("not found", http.StatusNotFound))
	}
}

func (app *app) contextFromRequest(w http.ResponseWriter, req *http.Request) Context {
	if ctx, ok := req.Context().Value(RequestContextKey).(Context); ok {
		return ctx
	}

	return newContext(app, w, req)
}
