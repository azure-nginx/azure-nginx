package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/azure-nginx/azure-nginx/common"
)

type NginxRequest struct {
	Name               string `json: "name"`
	NodeSKU            string `json: "nodeSku"`
	NodeCount          int    `json: "nodeCount"`
	ResourceGroup      string `json: "resourceGroup"`
	Location           string `json: "location"`
	CustomVNETSubnetID string `json: "customSubnetID"`
}

type NginxResponse struct {
	Success bool   `json: "success"`
	Error   string `json: "error"`
}

func (n *NginxRequest) Validate() error {
	if len(n.ResourceGroup) == 0 {
		return errors.New("ResourceGroup cannot be empty")
	} else if len(n.Location) == 0 {
		return errors.New("Location cannot be empty")
	} else if len(n.Name) == 0 {
		return errors.New("Name cannot be empty")
	} else if n.NodeCount <= 0 {
		return errors.New("NodeCount must be 1 or larger")
	}

	return nil
}

type NginxProvisioner struct {
	DeploymentManager *DeploymentManager
}

func (n *NginxProvisioner) Provision(w http.ResponseWriter, r *http.Request) {
	var request NginxRequest
	_ = json.NewDecoder(r.Body).Decode(&request)

	err := request.Validate()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	n.DeploymentManager.ResourceGroup = request.ResourceGroup
	n.DeploymentManager.Location = request.Location

	response, err := n.DeploymentManager.DeployNginx(request.Name, request.NodeSKU, request.NodeCount, request.CustomVNETSubnetID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (n *NginxProvisioner) Status(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status:": "running"})
}

func (n *NginxProvisioner) GetEndpoints() []common.Endpoint {
	return []common.Endpoint{{HTTPMethod: "POST", Endpoint: "/nginx", Func: n.Provision},
		{HTTPMethod: "GET", Endpoint: "/", Func: n.Status}}
}
