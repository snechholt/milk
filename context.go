package api

import (
	"appengine"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Context interface {
	appengine.Context

	// Next() calls the next handler in the chain of handlers, if any.
	// It can be used by middleware handlers to continue processing other handlers and
	// delay execution of code until after these have finished.
	Next()

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
	Stop()

	// R() returns the original *http.Request
	R() *http.Request

	// W() returns the original http.ResponseWriter
	W() http.ResponseWriter

	// SetResult sets the context's result, to be returned to the client
	SetResult(val interface{})

	// SetPagination sets the pagination information of the request
	SetPagination(pagination *Pagination)

	// Err() returns any errors returned by the handlers
	// If there are multiple errors, an error of type ErrorSlice is returned,
	// allowing access to each individual error in the order that they were generated
	Err() error

	// Params() returns the parameters of the request
	Params() Params

	// Vals() returns context specific values
	Vals() Values

	// ParseBody() parses the request body into the given struct pointer
	ParseBody(dst interface{}) error
}

type Params interface {
	Get(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetDate(key string) time.Time
}

type Values interface {
	Get(key interface{}) interface{}
	Set(key interface{}, val interface{})
	GetString(key interface{}) string
	GetInt(key interface{}) int
	GetInt64(key interface{}) int64
}

type Pagination struct {
	Next *url.URL
}

type ErrorSlice []error

func (this ErrorSlice) Error() string {
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

func GetBaseContext(c Context) appengine.Context {
	if ctx, ok := c.(*context); ok {
		return ctx.Context
	} else {
		return nil
	}
}

type context struct {
	appengine.Context
	r *http.Request
	w *responseWriter

	handlers []HandlerFunc
	index    int

	result     interface{}
	errs       ErrorSlice
	pagination *Pagination

	params *params
	vals   values
}

func newContext(c appengine.Context, r *http.Request, w http.ResponseWriter, p httprouter.Params, handlers []HandlerFunc) *context {
	return &context{
		Context:  c,
		r:        r,
		w:        &responseWriter{w: w},
		handlers: handlers,
		params:   &params{r: r, p: p},
		vals:     make(map[interface{}]interface{}),
	}
}

func (this *context) R() *http.Request       { return this.r }
func (this *context) W() http.ResponseWriter { return this.w }

func (this *context) Err() error {
	switch len(this.errs) {
	case 0:
		return nil
	case 1:
		return this.errs[0]
	default:
		return this.errs
	}
}

func (this *context) Params() Params {
	return this.params
}

func (this *context) Vals() Values {
	return this.vals
}

// ParseBody parses the body of the request as a JSON string and unmarshals it into dst.
func (this *context) ParseBody(dst interface{}) error {
	if b, err := ioutil.ReadAll(this.r.Body); err != nil {
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

func (this *context) SetResult(val interface{}) {
	this.result = val
}

func (this *context) SetPagination(p *Pagination) {
	this.pagination = p
}

func (this *context) Next() {
	if this.index >= len(this.handlers) {
		return
	}
	handler := this.handlers[this.index]
	this.index += 1

	if this.do(handler) {
		this.Next()
	} else {
		this.Stop()
	}
}

func (this *context) Stop() {
	this.index = len(this.handlers)
}

// do() executes the handler and returns whether or not to call the
// next handler in the chain
func (this *context) do(handler HandlerFunc) bool {
	if err := handler(this); err != nil {
		this.errs = append(this.errs, err)
		return false
	} else {
		return true
	}
}

// respond() sends a response based on the error and result set by the handlers.
// If there are any errors, respond() checks to see if it is an (API) Error or ValidationError and
// returns a non 500 status code response based on the error's status code and type. If not, a 500
// status code is returned.
// If there are no errors, the context's result is JSON encoded and written to the response writer.
// If any of the handlers have written to the context's ResponseWriter, respond() does nothing.
func (this *context) respond() {
	if this.w.written {
		// if any handler has written to the writer already, return
		return
	}

	w := this.W()
	var statusCode int

	if err := this.Err(); err != nil {

		this.SetResult(nil)

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
			this.SetResult(&s)
		} else if apierr, ok := err.(*Error); ok {
			statusCode = apierr.StatusCode
			if apierr.Message != "" {
				this.SetResult(apierr)
			}
		} else {
			statusCode = http.StatusInternalServerError
		}

	} else {
		statusCode = http.StatusOK
		if this.pagination != nil {
			w.Header().Set("X-Pagination-Next", this.pagination.Next.String())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if this.result != nil {
		if b, err := json.Marshal(this.result); err != nil {
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
