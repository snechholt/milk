package api

import (
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"net/http"
)

type HandlerFunc func(c *Context) error

type CreateContextFn func(r *http.Request) context.Context

type Router struct {
	CreateContext CreateContextFn
	parent        *Router
	r             *httprouter.Router
	path          string
	mw            []HandlerFunc
}

type notfound struct {
}

func (this *notfound) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
}

func NewRouter() *Router {
	r := httprouter.New()
	r.NotFound = new(notfound)
	r.MethodNotAllowed = new(notfound)
	return &Router{
		CreateContext: func(r *http.Request) context.Context { return appengine.NewContext(r) },
		r:             r,
	}
}

func (this *Router) router() *httprouter.Router {
	if this.r != nil {
		return this.r
	} else {
		return this.parent.router()
	}
}

func (this *Router) middleware() []HandlerFunc {
	var fns []HandlerFunc
	if this.parent != nil {
		fns = this.parent.middleware()
	}
	fns = append(fns, this.mw...)
	return fns
}

func (this *Router) createContext(r *http.Request) context.Context {
	if this.CreateContext != nil {
		return this.CreateContext(r)
	} else if this.parent != nil {
		return this.parent.createContext(r)
	} else {
		panic("No CreateContext func is set")
	}
}

func (this *Router) SubRouter(path string) *Router {
	sub := &Router{
		parent: this,
		path:   this.path + path,
	}
	return sub
}

func (this *Router) route(method, path string, handlers ...HandlerFunc) {
	// if path[0] != '/' {
	// 	panic("path must begin with '/' in path '" + path + "'") // taken directly from httprouter
	// }
	fns := this.middleware()
	fns = append(fns, handlers...)
	this.router().Handle(method, this.path+path, wrap(this.createContext, fns...))
}

func (this *Router) Get(path string, fns ...HandlerFunc)    { this.route("GET", path, fns...) }
func (this *Router) Post(path string, fns ...HandlerFunc)   { this.route("POST", path, fns...) }
func (this *Router) Put(path string, fns ...HandlerFunc)    { this.route("PUT", path, fns...) }
func (this *Router) Delete(path string, fns ...HandlerFunc) { this.route("DELETE", path, fns...) }

func (this *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	this.r.ServeHTTP(w, r)
}

func (this *Router) Use(middleware HandlerFunc) {
	this.mw = append(this.mw, middleware)
}

func wrap(createContext CreateContextFn, handlers ...HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		c := createContext(r)
		context := newContext(c, r, w, p, handlers)
		// Fire off the first handler by calling Next(). Next then calls itself recursively
		context.Next()
		// Create and send response
		context.respond()
	}
}
