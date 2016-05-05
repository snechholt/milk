package api

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
	"time"
)

var DateFormat = "2006-01-02"

type params struct {
	r *http.Request
	p httprouter.Params
}

// Get returns the given key's value from the request path parameters or querystring.
// The request path is searched first, and overrides any querystring values with the same key.
func (this *params) Get(key string) string {
	if val := this.p.ByName(key); val != "" {
		return val
	} else {
		return this.r.URL.Query().Get(key)
	}
}

// GetInt64 returns the given key's value as an int.
// Returns 0 for invalid or missing values.
func (this *params) GetInt(key string) int {
	if strVal := this.Get(key); strVal != "" {
		if intVal, err := strconv.Atoi(strVal); err == nil {
			return intVal
		}
	}
	return 0
}

// GetInt64 returns the given key's value as an int64.
// Returns 0 for invalid or missing values.
func (this *params) GetInt64(key string) int64 {
	var val int64
	if strVal := this.Get(key); strVal != "" {
		if intVal, err := strconv.ParseInt(strVal, 10, 64); err == nil {
			val = intVal
		}
	}
	return val
}

func (this *params) GetDate(key string) time.Time {
	t, _ := time.Parse(DateFormat, this.Get(key))
	return t
}
