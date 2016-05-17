package api

import (
	"appengine"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/http"
)

// Errors holds one or more errors returned by the handler functions executed on the context.
// The errors are arranged ordered by when they occured.
type Errors []error

func (this Errors) Error() string {
	switch len(this) {
	case 0:
		return "(no errors)"
	case 1:
		return this[0].Error()
	default:
		var buf bytes.Buffer
		buf.WriteString("Multiple handlers returned error:\n")
		for i, err := range this {
			buf.WriteString(fmt.Sprintf("Error #%d\n", i+1))
			buf.WriteString(err.Error())
			buf.WriteByte('\n')
		}
		return buf.String()
	}
}

type Context struct {
	appengine.Context

	// R is the original http request object of the handler
	R *http.Request

	// W will write to the original response writer of the handler
	W http.ResponseWriter

	// Result holds the data to be returned to the client
	Result interface{}

	// Params holds the parameters of the url and querystring of the context's request.
	Params *Params

	// Values holds context specific values
	Values Values

	w *responseWriter // w is a responseWriter wrapping W

	handlers []HandlerFunc // handlers is a slice of registered handlers to be run for the current request
	index    int           // index is the index of the current handler being processed in the handlers slice

	errs Errors
}

func newContext(c appengine.Context, r *http.Request, w http.ResponseWriter, p httprouter.Params, handlers []HandlerFunc) *Context {
	rw := &responseWriter{w: w}
	return &Context{
		Context:  c,
		R:        r,
		W:        rw,
		Params:   &Params{r: r, p: p},
		Values:   make(map[interface{}]interface{}),
		w:        rw,
		handlers: handlers,
	}
}

// ParseBody parses the body of the request as a JSON string and unmarshals it into dst.
func (this *Context) ParseBody(dst interface{}) error {
	if b, err := ioutil.ReadAll(this.R.Body); err != nil {
		return fmt.Errorf("error reading request body: %v", err)
	} else {
		if err = json.Unmarshal(b, dst); err != nil {
			this.Debugf("Error unmashalling body: %v", err)
			return ErrBadRequest
		} else {
			return nil
		}
	}
}

// Err() returns any errors returned by the handlers
// If there are multiple errors, an error of type Errors is returned,
// allowing access to each individual error in the order that they were generated.
func (this *Context) Err() error {
	switch len(this.errs) {
	case 0:
		return nil
	case 1:
		return this.errs[0]
	default:
		return this.errs
	}
}

// Next() calls the next handler in the chain of handlers, if any.
// It can be used by middleware handlers to continue processing other handlers and
// delay execution of code until after these have finished.
func (this *Context) Next() {
	if this.index >= len(this.handlers) {
		return
	}
	handler := this.handlers[this.index]
	this.index += 1

	if err := handler(this); err != nil {
		this.errs = append(this.errs, err)
		this.Stop()
	} else if this.w.written {
		this.Stop()
	} else {
		this.Next()
	}
}

// Stop() stops the context from calling any remaining handlers that have not yet run.
// Note that handlers that have already run before the handler calling Stop() and that have
// called Next() on the context will still execute the code that comes after Next().
//
// Example:
// func Middleware(c Context) error {
//	c.Next() // the next handler calls c.Stop()
//	c.Debugf("Hello") // this will still get called
//	return nil
//}
func (this *Context) Stop() {
	this.index = len(this.handlers)
}

// respond() sends a response based on the error and result set by the handlers.
// If there are any errors, respond() checks to see if it is an (API) Error or ValidationError and
// returns a non 500 status code response based on the error's status code and type. If not, a 500
// status code is returned.
// If there are no errors, the context's result is JSON encoded and written to the response writer.
// If any of the handlers have written to the context's ResponseWriter, respond() does nothing.
func (this *Context) respond() {
	if this.w.written {
		// if any handler has written to the writer already, return
		return
	}

	w := this.W
	var statusCode int

	if err := this.Err(); err != nil {

		this.Result = nil

		if verr, ok := err.(*ValidationError); ok {
			statusCode = StatusValidationError
			s := struct {
				StatusCode int           `json:"statusCode"`
				ErrorCode  string        `json:"errorCode"`
				Message    string        `json:"message"`
				Errors     []*FieldError `json:"errors"`
			}{
				StatusValidationError,
				"multi",
				"Validation error. See errors array for details.",
				verr.Errors,
			}
			this.Result = &s
		} else if apierr, ok := err.(*Error); ok {
			statusCode = apierr.StatusCode
			if apierr.Message != "" {
				this.Result = apierr
			}
		} else {
			statusCode = http.StatusInternalServerError
		}

	} else {
		statusCode = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	if this.Result != nil {
		if b, err := json.Marshal(this.Result); err != nil {
			this.Errorf("json.Marshal error of response body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(statusCode)
			w.Write(b)
		}
	} else {
		w.WriteHeader(statusCode)
	}
}

// responseWriter wraps a http.ResponseWriter and tracks whether or not Write() or WriteHeader() has been called
type responseWriter struct {
	w       http.ResponseWriter
	written bool // written indicates
}

func (this *responseWriter) Header() http.Header {
	return this.w.Header()
}

func (this *responseWriter) Write(b []byte) (int, error) {
	this.written = true
	return this.w.Write(b)
}

func (this *responseWriter) WriteHeader(statusCode int) {
	this.written = true
	this.w.WriteHeader(statusCode)
}
