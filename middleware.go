package api

import (
	"strings"
)

func AllowAllCORS(c Context) error {
	r, w := c.R(), c.W()
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	return nil
}

func CORSPreflight(methods []string, headers []string) HandlerFunc {
	return func(c Context) error {
		r, w := c.R(), c.W()
		// CORS preflight requests
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))
			w.WriteHeader(200)
		}
		return nil
	}
}
