package hang

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"net/http"
	"os"
	"strings"
	"os/signal"
	"syscall"
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
	c chan os.Signal
	ExecName string
	ProcessName string
}

func NewHandler(lg Logger, processName string) *Handler {
	if lg == nil {
		lg = &logrus.Logger{
			Out:       os.Stderr,
			Formatter: new(logrus.TextFormatter),
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.DebugLevel,
		}
	}
	h := &Handler{}

	// Log app sigterm (stop by the user - killing can't be catched)
	h.c = make(chan os.Signal, 1)
	signal.Notify(h.c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go h.WaitForShutdownCleaning()

	h.ExecName = os.Args[0]

	// Can be set by the user
	if processName == "" {
		h.ProcessName = h.ExecName
	} else {
		h.ProcessName = processName
	}

	h.Log = lg

	h.Log.Infof("%v: started", h.ProcessName)

	h.Routes = map[string]HandleFunc{}
	h.AddRoute("default", h.RouteNotSet)
	h.AddRoute("livecheck", h.LiveCheck)

	return h
}

func (h *Handler) SetProcessName(name string) {
	h.ProcessName = name
}

func (h *Handler) RouteNotSet(resp http.ResponseWriter, req *http.Request) error {
	path := strings.Replace(req.URL.Path, "/", "", -1)
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write([]byte("Route not found: " + path))
	h.Log.Debug("Route not found: " + path)
	return nil
}

func (h *Handler) LiveCheck(resp http.ResponseWriter, req *http.Request) error {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("OK"))
	h.Log.Debug("LiveCheck invoked")
	return nil
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
		err error
	)
	// Find the route requested
	path = strings.Replace(req.URL.Path, "/", "", -1)
	logrus.Println("Path: ", path)
	handled = false
	for route, handler = range h.Routes {
		if path == route {
			err = handler(resp, req)
			if err != nil {
				h.Log.Debug(err)
			}
			handled = true
			break
		}
	}
	if !handled {
		h.Routes["default"](resp, req)
	}
}

func (h *Handler) WaitForShutdownCleaning() {
	// Waiting for exit signal on the channel
	<-h.c

	h.Log.Infof("%v: stopped by the user", h.ProcessName)
	os.Exit(0)
}
