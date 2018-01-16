package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/azure-nginx/azure-nginx/common"
	"github.com/gorilla/mux"
)

type App struct {
	Router       *mux.Router
	ControlPlane *NginxControlPlane
}

var httpPort = "80"
var token = ""

func (a *App) Run() {
	token = a.GetAPIToken()
	a.Router = mux.NewRouter()
	a.initRoutes()
	a.ControlPlane.Init()

	http.Handle("/", a.Router)
	common.Log.Fatal(http.ListenAndServe(":"+httpPort, a.Router))
}

func (a *App) GetAPIToken() string {
	contents, err := ioutil.ReadFile("/var/lib/controlplane/apitoken.txt")
	if err != nil {
		error := "API token not found"
		common.Log.Println(error)
		panic(error)
	}

	return string(contents)
}

func (a *App) initRoutes() {
	for _, endpoint := range a.ControlPlane.GetEndpoints() {
		a.Router.Handle(endpoint.Endpoint, Middleware(http.HandlerFunc(endpoint.Func))).Methods(endpoint.HTTPMethod)
	}
}

func Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerToken := r.Header.Get("token")
		if headerToken == token {
			h.ServeHTTP(w, r)
		} else {
			respondWithError(w, http.StatusUnauthorized, "missing or invalid token")
		}
	})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithFile(w http.ResponseWriter, contents []byte, filename string) {
	b := bytes.NewBuffer(contents)

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")

	b.WriteTo(w)
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
