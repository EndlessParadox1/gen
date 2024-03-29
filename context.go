package gen

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
)

const (
	MIMEJSON      = "application/json"
	MIMEHTML      = "text/html"
	MIMEPlain     = "text/plain"
	MIMEPOSTForm  = "application/x-www-form-urlencoded"
	MIMEMultipart = "multipart/form-data"
)

type H map[string]any

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	Path   string
	Method string
	Params httprouter.Params

	handlers []HandlerFunc
	index    int
	engine   *Engine

	mu         sync.RWMutex // protects Keys
	Keys       map[string]any
	StatusCode int
	Errors     []*error // Errors is a list of errors attached to all the handlers/middlewares
}

func newContext(w http.ResponseWriter, req *http.Request, params httprouter.Params) *Context {
	return &Context{
		Writer:  w,
		Request: req,
		Path:    req.URL.Path,
		Method:  req.Method,
		Params:  params,
		index:   -1,
	}
}

// Copy returns a copy of the current context that can be safely used outside the request's scope.
// This has to be used when the context has to be passed to a goroutine.
func (c *Context) Copy() *Context {
	cp := Context{
		Request: c.Request,
		Path:    c.Path,
		Method:  c.Method,
		index:   len(c.handlers),
		engine:  c.engine,
	}
	cp.Keys = make(map[string]any, len(c.Keys))
	c.mu.RLock()
	for k, v := range c.Keys {
		cp.Keys[k] = v
	}
	c.mu.RUnlock()
	cp.Params = make(httprouter.Params, len(c.Params))
	copy(cp.Params, c.Params)
	return &cp
}

// HandlerName returns the main handler's name
func (c *Context) HandlerName() string {
	len_ := len(c.handlers)
	f := c.handlers[len_-1]
	name := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	return name
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort Note that this will not stop the current handler, often followed by `return`
func (c *Context) Abort() {
	c.index = len(c.handlers)
}

func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

func (c *Context) RemoteIP() string {
	ip, _, _ := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	return ip
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	} // lazy init
	c.Keys[key] = value
}

func (c *Context) Get(key string) (value any, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok = c.Keys[key]
	return
}

func (c *Context) MustGet(key string) any {
	if value, ok := c.Get(key); ok {
		return value
	}
	panic("Key \"" + key + "\" does not exist!")
}

/**********************/
/******** INPUT *******/
/**********************/

func (c *Context) Param(key string) string {
	value := c.Param(key)
	return value
}

// PostForm for x-www-form-urlencoded POST
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val, nil
}

/***********************/
/******** OUTPUT *******/
/***********************/

func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) String(code int, format string, a ...any) {
	c.SetHeader("Content-Type", "text/plain")
	c.Status(code)
	c.Writer.Write([]byte(fmt.Sprintf(format, a...)))
}

func (c *Context) JSON(code int, obj any) {
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		panic(err)
	}
}

func (c *Context) HTML(code int, name string, data any) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
		panic(err)
	}
}

func (c *Context) Data(code int, contentType string, data []byte) {
	c.SetHeader("Content-Type", contentType)
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) File(filePath string) {
	http.ServeFile(c.Writer, c.Request, filePath)
}

func (c *Context) Redirect(location string) {
	http.Redirect(c.Writer, c.Request, location, http.StatusMovedPermanently)
}

func (c *Context) SetCookie(
	name string,
	value string,
	maxAge int,
	path string,
	domain string,
	secure bool,
	httpOnly bool,
) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}
