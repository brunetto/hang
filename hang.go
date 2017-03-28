package hang

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"syscall"
)

// Logger defines which methods are requested for a logger to be used in this package
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

// HandleFunc is the type of function to be used to handle a route in this package
type HandleFunc func(http.ResponseWriter, *http.Request) error

// Handler contains the core data of the generic handler: logger, routes, ...
type Handler struct {
	// Logger to be used
	Log         Logger
	// Map to match a route with the correct handler
	Routes      map[string]HandleFunc
	// Channel to listen for quit signal
	c           chan os.Signal
	// Name of the called process
	ExecName    string
	// Nice name of the service, given by the user
	ProcessName string
}

// NewHandler provides a new, initialized, generic handler
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
	go h.WaitForShutdown()

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

// SetProcessName sets the nice user defined service name
func (h *Handler) SetProcessName(name string) {
	h.ProcessName = name
}

// RouteNotSet is the default handler for routes with no handler registered
func (h *Handler) RouteNotSet(resp http.ResponseWriter, req *http.Request) error {
	path := strings.Replace(req.URL.Path, "/", "", -1)
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write([]byte("Route not found: " + path))
	h.Log.Debug("Route not found: " + path)
	return nil
}

// LiveCheck is the minimum healthy check for the service
func (h *Handler) LiveCheck(resp http.ResponseWriter, req *http.Request) error {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("OK"))
	h.Log.Debug("LiveCheck invoked")
	return nil
}

// AddRoute registers a handler for a route
func (h *Handler) AddRoute(route string, handleFunc HandleFunc) error {
	// If route already exists fire an error
	if _, exists := h.Routes[route]; exists {
		return errors.New("Route " + route + " already exists.")
	}
	h.Routes[route] = handleFunc
	return nil
}

// DeleteRoute unregister a route
func (h *Handler) DeleteRoute(route string) {
	delete(h.Routes, route)
}

// ModifyRoute registers a new handler for a route
func (h *Handler) ModifyRoute(route string, handleFunc HandleFunc) error {
	if _, exists := h.Routes[route]; !exists {
		return errors.New("Route " + route + "does not exists.")
	}
	h.Routes[route] = handleFunc
	return nil
}

// Handle takes care of routing the request to the right handler
func (h *Handler) Handle(resp http.ResponseWriter, req *http.Request) {
	var (
		path    string
		route   string
		handler HandleFunc
		handled bool
		err     error
	)
	// Find the route requested
	path = strings.Replace(req.URL.Path, "/", "", -1)
	handled = false
	for route, handler = range h.Routes {
		if path == route {
			h.Log.WithFields(logrus.Fields{"route": route, "function": GetFunctionName(handler)}).Debug()
			err = handler(resp, req)
			if err != nil {
				h.Log.WithFields(logrus.Fields{"route": route, "function": GetFunctionName(handler)}).Info(err)
			}
			handled = true
			break
		}
	}
	if !handled {
		h.Routes["default"](resp, req)
	}
}

// WaitForShutdown waits the quit signal
func (h *Handler) WaitForShutdown() {
	// Waiting for exit signal on the channel
	<-h.c

	h.Log.Infof("%v: stopped by the user", h.ProcessName)
	os.Exit(0)
}

// GetFunctionName returns the function name for debugging purposes
func GetFunctionName(handler HandleFunc) string {
	return runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
}
