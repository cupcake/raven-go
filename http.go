package raven

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
)

// NewHttp creates new HTTP object that follows Sentry's HTTP interface spec and will be attached to the Packet
func NewHttp(req *http.Request) *Http {
	proto := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	h := &Http{
		Method:  req.Method,
		Cookies: req.Header.Get("Cookie"),
		Query:   sanitizeQuery(req.URL.Query()).Encode(),
		URL:     proto + "://" + req.Host + req.URL.Path,
		Headers: make(map[string]string, len(req.Header)),
	}
	if addr, port, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		h.Env = map[string]string{"REMOTE_ADDR": addr, "REMOTE_PORT": port}
	}
	for k, v := range req.Header {
		h.Headers[k] = strings.Join(v, ",")
	}
	h.Headers["Host"] = req.Host
	return h
}

var querySecretFields = []string{"password", "passphrase", "passwd", "secret"}

func sanitizeQuery(query url.Values) url.Values {
	for _, keyword := range querySecretFields {
		for field := range query {
			if strings.Contains(field, keyword) {
				query[field] = []string{"********"}
			}
		}
	}
	return query
}

// Http defines Sentry's spec compliant interface holding Request information - https://docs.sentry.io/development/sdk-dev/interfaces/http/
type Http struct {
	// Required
	URL    string `json:"url"`
	Method string `json:"method"`
	Query  string `json:"query_string,omitempty"`

	// Optional
	Cookies string            `json:"cookies,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// Must be either a string or map[string]string
	Data interface{} `json:"data,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (h *Http) Class() string { return "request" }

// RecoveryHandler uses Recoverer to wrap the stdlib net/http Mux.
// Example:
//	http.HandleFunc("/", raven.RecoveryHandler(func(w http.ResponseWriter, r *http.Request) {
//		...
//	}))
func RecoveryHandler(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return Recoverer(http.HandlerFunc(handler)).ServeHTTP
}

// Recoverer wraps the stdlib net/http Mux.
// Example:
//  mux := http.NewServeMux
//  ...
//	http.Handle("/", raven.Recoverer(mux))
func Recoverer(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rval := recover(); rval != nil {
				log.Print(rval)
				debug.PrintStack()
				rvalStr := fmt.Sprint(rval)
				var packet *Packet
				if err, ok := rval.(error); ok {
					packet = NewPacket(rvalStr, NewException(errors.New(rvalStr), GetOrNewStacktrace(err, 2, 3, nil)), NewHttp(r))
				} else {
					packet = NewPacket(rvalStr, NewException(errors.New(rvalStr), NewStacktrace(2, 3, nil)), NewHttp(r))
				}
				Capture(packet, nil)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		handler.ServeHTTP(w, r)
	})
}
