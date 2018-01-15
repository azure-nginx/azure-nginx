package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type App struct {
	Router           *mux.Router
	NginxProvisioner *NginxProvisioner
}

var httpPort string = "8080"

func (a *App) Run(port string, appId string, clientSecret string, tenantId string, subscriptionId string) {
	if len(port) != 0 {
		httpPort = port
	}

	if len(appId) == 0 {
		panic("APP_ID env var is null")
	} else if len(clientSecret) == 0 {
		panic("CLIENT_SECRET env var is null")
	} else if len(tenantId) == 0 {
		panic("TENANT_ID env var is null")
	} else if len(subscriptionId) == 0 {
		panic("SUBSCRIPTION_ID env var is null")
	}

	deploymentManager := DeploymentManager{AppID: appId, ClientSecret: clientSecret, TenantID: tenantId, SubscriptionID: subscriptionId}
	deploymentManager.Init()

	a.NginxProvisioner = &NginxProvisioner{DeploymentManager: &deploymentManager}

	a.Router = mux.NewRouter()
	a.initRoutes()

	log.Fatal(http.ListenAndServe(":"+httpPort, a.Router))
}

func (a *App) initRoutes() {
	for _, endpoint := range a.NginxProvisioner.GetEndpoints() {
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
