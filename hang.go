package hang

import (
	"github.com/pkg/errors"
	"github.com/Sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"path/filepath"
	"gitlab.com/brunetto/ritter"
	"io/ioutil"
	"encoding/json"
	"bytes"
	"io"
	"gitlab.com/brunetto/swaggo"
	"gopkg.in/gin-gonic/gin.v1"
	"github.com/brunetto/gin-logrus"
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
	path := GetRoute(req)
	resp.WriteHeader(http.StatusBadRequest)
	resp.Write([]byte("Route not found: " + path))
	h.Log.WithFields(logrus.Fields{"origin": req.RemoteAddr}).Info("Route not found: " + path)
	return nil
}

// LiveCheck is the minimum healthy check for the service
func (h *Handler) LiveCheck(resp http.ResponseWriter, req *http.Request) error {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("OK"))
	h.Log.WithFields(logrus.Fields{"origin": req.RemoteAddr}).Debug("LiveCheck invoked")
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
	path = GetRoute(req)
	handled = false
	for route, handler = range h.Routes {
		if path == route {
			h.Log.WithFields(logrus.Fields{"route": route, "function": GetFunctionName(handler),"origin": req.RemoteAddr}).Debug()
			err = handler(resp, req)
			if err != nil {
				h.Log.WithFields(logrus.Fields{"route": route, "function": GetFunctionName(handler), "origin": req.RemoteAddr}).Error(err)
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

// WaitForShutdown waits the quit signal
func LogStartAndStop(processName string, logger Logger) {
	// Create signal channel
	c := make(chan os.Signal, 1)
	// Catch stop signals and send them to the channel
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	// Sping goroutine
	go func(c chan os.Signal, processName string, logger Logger) {
		// Waiting for exit signal on the channel
		<-c

		logger.Infof("%v: stopped by the user", processName)
		os.Exit(0)
	}(c, processName, logger)

	logger.Infof("%v: started", processName)
}

// GetFunctionName returns the function name for debugging purposes
func GetFunctionName(handler HandleFunc) string {
	return filepath.Base(runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name())
}

func GetRoute (req *http.Request) string {
	//return strings.Replace(req.URL.Path, "/", "", -1)
	return strings.TrimLeft(strings.TrimRight(req.URL.Path, "/"), "/")
}

//type LogFields logrus.Fields//map[string]interface{} // same as logrus.Fields

func At() logrus.Fields {
	return logrus.Fields{"at": Here()}
}

func Here() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	me := runtime.FuncForPC(pc)
	if me == nil {
		return "unnamed"
	}
	return filepath.Base(me.Name())
}


func NewDefaultLogger() Logger {
	var (
		rotatedWriter *ritter.Writer
		err           error
	)
	// New writer with rotation
	rotatedWriter, err = ritter.NewRitterTime("default.log")
	if err != nil {
		logrus.Fatal("can't create log file: " + err.Error())
	}

	// Tee to stderr
	rotatedWriter.TeeToStdErr = true

	//logFormatter := new(logrus.TextFormatter)
	//logFormatter.FullTimestamp = true

	// Create logger
	lg := (&logrus.Logger{
		Out: rotatedWriter,
		//Formatter: logFormatter,
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}).WithFields(logrus.Fields{
		"url": "syncer.udctracker.pixartprinting.local",
	})
	return lg
}

func ChooseLogLevel(level string) logrus.Level {
	switch level {
	case "info", "Info":
		return logrus.InfoLevel
	case "debug", "Debug":
		return logrus.DebugLevel
	case "warn", "Warn", "warning", "Warning":
		return logrus.WarnLevel
	case "err", "Err", "error", "Error":
		return logrus.ErrorLevel
	case "fatal", "Fatal":
		return logrus.FatalLevel
	case "panic", "Panic":
		return logrus.PanicLevel
	default:
		return logrus.DebugLevel
	}
}

func GetReqData(resp http.ResponseWriter, req *http.Request) ([]byte, error) {
	var (
		err error
		body []byte
	)

	// Check the request contains data
	if req.Body == nil {
		// Generate error
		err = errors.New("Missing input data")
		// Send response
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte(err.Error()))
		// Exit
		return body, err
	}

	// Extract
	body, err = ioutil.ReadAll(req.Body)
	if err != nil {
		if err.Error() == "EOF" {
			// Wrap error
			err = errors.Wrap(err, "EOF error reading JSON, maybe you are trying to read again an already processed response body")
			// Respond
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
			// Exit
			return body, err
		} else {
			// Wrap error
			err = errors.Wrap(err, "error reading request body")
			// Respond
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			// Exit
			return body, err
		}
	}
	req.Body.Close()
	// Return
	return body, err
}

func GetReqJSONData(resp http.ResponseWriter, req *http.Request, data interface{}) error {
	var (
		body []byte
		err error
	)
	body, err = GetReqData(resp, req)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, data)
	if err != nil {
		err = errors.Wrap(err, "can't decode input JSON")
		// Respond
		if resp != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
		}
		return err
	}
	return nil
}

func Tee(httpReqBody *io.ReadCloser) []byte {
	var b []byte
	b, _ = ioutil.ReadAll(*httpReqBody)
	*httpReqBody = ioutil.NopCloser(bytes.NewBuffer(b))
	return b
}

func GinOnTheRocks(appName string) (*gin.Engine, *swaggo.Swaggo, Logger, error) {
	var (
		err           error
		rotatedWriter *ritter.Writer
		r             *gin.Engine
		s *swaggo.Swaggo
		log Logger
	)
	// NewMonitor writer with rotation
	rotatedWriter, err = ritter.NewRitterTime("storage/logs/" + appName + ".log")
	if err != nil {
		return r, s, log, errors.Wrap(err, "can't create log file")
	}

	// Tee to stderr
	rotatedWriter.TeeToStdErr = true

	// Create logger
	log = &logrus.Logger{
		Out:   rotatedWriter,
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
		Formatter: new(logrus.JSONFormatter),
	}

	// New engine
	r = gin.New()
	r.Use(ginlogrus.Logger(log.(*logrus.Logger)), gin.Recovery())

	// Swagger addDocs with redoc UI
	s, err = swaggo.NewSwaggo()
	if err != nil {
		return r, s, log, errors.Wrap(err, "can't create new swaggo")
	}

	s.AddUndocPaths("favicon")
	s.AddEndpoint("/livecheck", "GET", "",
		swaggo.Response(http.StatusOK, "", "Service is alive"),
		swaggo.Description("Endpoint to ensure service is up and running"),
		swaggo.Consumes(""),
		swaggo.Produces("text/plain"),
	)
	s.AddEndpoint("/livecheck", "POST", "",
		swaggo.Response(http.StatusOK, "", "Service is alive"),
		swaggo.Description("Endpoint to ensure service is up and running"),
		swaggo.Consumes(""),
		swaggo.Produces("text/plain"),
	)

	LogStartAndStop(appName, log)
	return r, s, log, err
}
