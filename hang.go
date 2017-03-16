package hang

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"net/http"
	"os"
	"strings"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugln(args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorln(args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalln(args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infoln(args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicln(args ...interface{})
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Println(args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Warningln(args ...interface{})
	Warnln(args ...interface{})
	WithFields(fields logrus.Fields) *logrus.Entry
}

type HandleFunc func(http.ResponseWriter, *http.Request) error

type Handler struct {
	Log    Logger
	Routes map[string]HandleFunc
}

func NewHandler(lg Logger) *Handler {
	if lg == nil {
		lg = &logrus.Logger{
			Out:       os.Stderr,
			Formatter: new(logrus.TextFormatter),
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.DebugLevel,
		}
	}
	h := &Handler{}
	h.Log = lg

	h.Routes = map[string]HandleFunc{}
	h.AddRoute("default", h.RouteNotSet)

	return h
}

func (h *Handler) RouteNotSet(resp http.ResponseWriter, req *http.Request) error {
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write([]byte("Route not found"))
	h.Log.Debug("Route not found")
	return nil
}

func (h *Handler) LiveCheck(resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("OK"))
	h.Log.Debug("LiveCheck invoked")
	return
}

func (h *Handler) AddRoute(route string, handleFunc HandleFunc) error {
	// If route already exists fire an error
	if _, exists := h.Routes[route]; exists {
		return errors.New("Route " + route + " already exists.")
	}
	h.Routes[route] = handleFunc
	return nil
}

func (h *Handler) DeleteRoute(route string) {
	delete(h.Routes, route)
}

func (h *Handler) ModifyRoute(route string, handleFunc HandleFunc) error {
	if _, exists := h.Routes[route]; !exists {
		return errors.New("Route " + route + "does not exists.")
	}
	h.Routes[route] = handleFunc
	return nil
}

func (h *Handler) Handle(resp http.ResponseWriter, req *http.Request) {
	var (
		path    string
		route   string
		handler HandleFunc
		handled bool
	)
	// Find the route requested
	path = strings.ToLower(strings.Replace(req.URL.Path, "/", "", -1))
	handled = false
	for route, handler = range h.Routes {
		if path == route {
			handler(resp, req)
			handled = true
			break
		}
	}
	if !handled {
		h.Routes["default"](resp, req)
	}
}
