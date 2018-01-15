package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/azure-nginx/azure-nginx/common"
	"github.com/gorilla/mux"
)

type App struct {
	Router *mux.Router
	Agent  *NginxAgent
}

var httpPort = "4050"

func (a *App) Run() {
	a.Router = mux.NewRouter()
	a.initRoutes()
	a.Agent.MakeSureNginxLives()

	common.Log.Fatal(http.ListenAndServe(":"+httpPort, a.Router))
}

func (a *App) initRoutes() {
	for _, endpoint := range a.Agent.GetEndpoints() {
		a.Router.HandleFunc(endpoint.Endpoint, endpoint.Func).Methods(endpoint.HTTPMethod)
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.Encode(payload)

	bytes := buffer.Bytes()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(bytes)
}
