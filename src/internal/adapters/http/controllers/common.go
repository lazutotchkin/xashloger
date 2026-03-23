package controllers

import (
	"net/http"
	"strconv"
)

func parsePageParam(r *http.Request) int {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	return page
}
