package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/azure-nginx/azure-nginx/common"
)

type NginxControlPlane struct {
}

type Node struct {
	Address        string
	Healthy        bool
	UnhealthyCount int
	WaitingUpgrade bool
	IsUpgrading    bool
}

var (
	Nodes       []Node
	muxInstance sync.Mutex
)

type RegistrationRequest struct {
	NodeAddress string `json: "nodeAddress"`
}

func (n *NginxControlPlane) Init() {
	n.CheckIfNodesAreHealthy()
	n.UpgradeNodesIfNeeded()
}

func (n *NginxControlPlane) GetEndpoints() []common.Endpoint {
	return []common.Endpoint{{HTTPMethod: "POST", Endpoint: "/nodes/register", Func: n.NodeWantsToRegister},
		{HTTPMethod: "POST", Endpoint: "/upgrade", Func: n.MarkNodesForUpgrade},
		{HTTPMethod: "GET", Endpoint: "/upgrade/status", Func: n.GetUpgradeStatus},
		{HTTPMethod: "GET", Endpoint: "/nodes", Func: n.GetNodes},
		{HTTPMethod: "GET", Endpoint: "/configuration", Func: n.GetConfiguration},
		{HTTPMethod: "POST", Endpoint: "/configuration", Func: n.UpdateConfiguration}}
}

func (n *NginxControlPlane) GetConfiguration(w http.ResponseWriter, r *http.Request) {
	config := ""

	for _, node := range Nodes {
		if node.Healthy {
			response, err := http.Get("http://" + node.Address + "/configuration")
			if err == nil {
				responseData, _ := ioutil.ReadAll(response.Body)
				config = string(responseData)
				break
			}
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status:": "ok", "configuration": config})
}

func (n *NginxControlPlane) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]string{"status:": "error", "message": err.Error()})
		return
	}

	var buf bytes.Buffer
	io.Copy(&buf, file)

	dataBytes := buf.Bytes()
	reader := bytes.NewReader(dataBytes)

	for _, node := range Nodes {
		if node.Healthy {
			_, err := http.Post("http://"+node.Address+"/configuration", "application/octet-strea", reader)
			if err != nil {
				common.Log.Println("node " + node.Address + " failed to update configuration. Error: " + err.Error())
			}
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status:": "ok"})
}

func (n *NginxControlPlane) NodeWantsToRegister(w http.ResponseWriter, r *http.Request) {
	var request RegistrationRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	n.RegisterNode(request)
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (n *NginxControlPlane) GetUpgradeStatus(w http.ResponseWriter, r *http.Request) {
	upgradingNodesCount := 0

	for _, node := range Nodes {
		if node.IsUpgrading {
			upgradingNodesCount++
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status:": "ok", "message": strconv.Itoa(upgradingNodesCount) + " nodes are currently upgrading"})
}

func (n *NginxControlPlane) CheckIfNodesAreHealthy() {
	healthTicker := time.NewTicker(time.Second * 5)

	go func() {
		for _ = range healthTicker.C {
			for i := len(Nodes) - 1; i >= 0; i-- {
				node := Nodes[i]
				response, err := http.Get("http://" + node.Address + "/status")
				isHealthy := true

				if err != nil || response.StatusCode != 200 {
					isHealthy = false
					Nodes[i].UnhealthyCount++

					common.Log.Println("Node " + node.Address + " is unhealthy")
					common.Log.Println(err)

					if Nodes[i].UnhealthyCount > 100 {
						Nodes = append(Nodes[:1], Nodes[:1+i]...)
					}

				} else {
					Nodes[i].UnhealthyCount = 0
					common.Log.Println("Node " + node.Address + " is healthy")
				}

				Nodes[i].Healthy = isHealthy
			}
		}
	}()
}

func (n *NginxControlPlane) RegisterNode(request RegistrationRequest) {
	muxInstance.Lock()

	for _, node := range Nodes {
		if node.Address == request.NodeAddress {
			muxInstance.Unlock()
			return
		}
	}

	newNode := Node{Address: request.NodeAddress, Healthy: true}
	Nodes = append(Nodes, newNode)

	common.Log.Println("Node at address " + newNode.Address + " registered")
	muxInstance.Unlock()
}

func (n *NginxControlPlane) GetNodes(w http.ResponseWriter, r *http.Request) {
	nodesJSON, _ := json.Marshal(Nodes)
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok", "nodes": string(nodesJSON)})
}

func (n *NginxControlPlane) UpgradeNodesIfNeeded() {
	upgradeTicker := time.NewTicker(time.Second * 5)

	go func() {
		for _ = range upgradeTicker.C {
			for i, node := range Nodes {
				if node.WaitingUpgrade && !node.IsUpgrading {
					Nodes[i].IsUpgrading = true

					response, err := http.Post("http://"+node.Address+"/upgrade", "application/json", nil)
					Nodes[i].IsUpgrading = false

					if err != nil || response.StatusCode != 200 {
						common.Log.Println("Node " + node.Address + " failed to upgrade. Awaiting next batch")
					} else {
						Nodes[i].WaitingUpgrade = false
						common.Log.Println("Node " + node.Address + " upgraded successfully")
					}
				}
			}
		}
	}()
}

func (n *NginxControlPlane) MarkNodesForUpgrade(w http.ResponseWriter, r *http.Request) {
	for i := range Nodes {
		Nodes[i].WaitingUpgrade = true
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Upgrade started"})
}
