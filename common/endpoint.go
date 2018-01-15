package common

import "net/http"

type EndpointFunc func(w http.ResponseWriter, r *http.Request)

type Endpoint struct {
	HTTPMethod string
	Endpoint   string
	Func       EndpointFunc
}
