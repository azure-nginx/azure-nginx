package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/azure-nginx/azure-nginx/common"
)

type NginxAgent struct {
}

var (
	backupPath          = os.Getenv("HOME") + "/nginxagent"
	confPath            = "/etc/nginx/nginx.conf"
	controlPlaneAddress = ""
	nginxAgentPath      = "/var/lib/nginxagent"
	myHostname, _       = os.Hostname()
	upgradeCommands     = []string{"sudo add-apt-repository ppa:nginx/stable", "sudo apt-get update", "sudo apt-get -y install nginx"}
	isUpgrading         = false
)

func (n *NginxAgent) ReadControlPlaneAddress() {
	addr, _ := ioutil.ReadFile(nginxAgentPath + "/cp_config.txt")
	controlPlaneAddress = strings.TrimSpace(string(addr))
}

func (n *NginxAgent) RegisterWithControlPlane() {
	jsonData := map[string]string{"nodeAddress": myHostname + ":4050"}
	jsonValue, _ := json.Marshal(jsonData)

	cpAddress := "http://" + controlPlaneAddress + "/nodes/register"

	_, err := http.Post(cpAddress, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		common.Log.Println("HTTP call (Registration) to Control Plane failed: " + err.Error())
	}
}

func (n *NginxAgent) UpdateControlPlaneWithConfig() {
	jsonData := map[string]string{"configFile": myHostname}
	jsonValue, _ := json.Marshal(jsonData)

	_, err := http.Post("http://"+controlPlaneAddress, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		common.Log.Println("HTTP call (Config Update) to Control Plane failed: " + err.Error())
	}
}

func (n *NginxAgent) MakeSureNginxLives() {
	n.ReadControlPlaneAddress()
	n.RegisterWithControlPlane()
	n.CreateBackupDirectory()

	backupTicker := time.NewTicker(time.Second * 3)

	go func() {
		for _ = range backupTicker.C {
			n.PeriodicallyBackupConfig()
		}
	}()

	keepAliveTicker := time.NewTicker(time.Second * 5)

	go func() {
		for _ = range keepAliveTicker.C {
			if !isUpgrading {
				n.FixConfigIfNeeded()
				n.KeepTheProcessAlive()
			}
		}
	}()

	registrationTicker := time.NewTicker(time.Second * 20)

	go func() {
		for _ = range registrationTicker.C {
			n.RegisterWithControlPlane()
		}
	}()
}

func (n *NginxAgent) KeepTheProcessAlive() {
	var (
		cmdOut []byte
		err    error
	)

	cmd := "sudo service nginx status"
	if cmdOut, err = exec.Command("/bin/sh", "-c", cmd).Output(); err != nil || !strings.Contains(string(cmdOut), "active (running)") {
		common.Log.Println("Mayday! nginx seems to be down!")
		n.TryElectricShock()
	} else {
		common.Log.Println("nginx is alive & kickin'")
	}
}

func (n *NginxAgent) FixConfigIfNeeded() {
	err := n.IsConfigValid(confPath)
	if err != nil {
		if _, errOS := os.Stat(backupPath + "/nginx.conf"); errOS == nil {
			n.CopyGoodBackupToSource()
			n.TryElectricShock()
		} else {
			common.Log.Println(errOS)
		}
	}
}

func (n *NginxAgent) CopyGoodBackupToSource() {
	cmd := "sudo cp " + backupPath + "/nginx.conf " + confPath
	_, err := n.RunCustomCommand(cmd)

	if err != nil {
		common.Log.Println(err)
	}
}

func (n *NginxAgent) CopyCurrentConfig(dest string) {
	cmd := "sudo cp " + confPath + " " + dest
	n.RunCustomCommand(cmd)
}

func (n *NginxAgent) RunCustomCommand(cmd string) (string, error) {
	cmdOut, err := exec.Command("/bin/sh", "-c", cmd).Output()
	return string(cmdOut), err
}

func (n *NginxAgent) CreateBackupDirectory() {
	cmd := "sudo mkdir " + backupPath
	n.RunCustomCommand(cmd)
}

func (n *NginxAgent) IsConfigValid(configPath string) error {
	var err error
	var errb bytes.Buffer

	cmd := "sudo nginx -t"
	execCmd := exec.Command("/bin/sh", "-c", cmd)
	execCmd.Stderr = &errb

	err = execCmd.Run()

	if err != nil || !strings.Contains(string(errb.String()), "test is successful") {
		common.Log.Println("Mayday! config seems to be broken!")
		return errors.New("config broken")
	}

	common.Log.Println("nginx config is rock solid")
	return nil
}

func (n *NginxAgent) DeleteFile(path string) {
	cmd := "sudo rm -r -f " + path
	n.RunCustomCommand(cmd)
}

func (n *NginxAgent) PeriodicallyBackupConfig() {
	candidatePath := backupPath + "/nginx.conf.candidate"
	n.CopyCurrentConfig(candidatePath)

	err := n.IsConfigValid(candidatePath)
	n.DeleteFile(candidatePath)

	if err == nil {
		n.CopyCurrentConfig(backupPath + "/nginx.conf")
	} else {
		common.Log.Println(err)
	}
}

func (n *NginxAgent) TryElectricShock() {
	cmd := "sudo service nginx restart"
	n.RunCustomCommand(cmd)

	common.Log.Println("Tried electric shock - " + cmd)
}

func (n *NginxAgent) GetEndpoints() []common.Endpoint {
	return []common.Endpoint{{HTTPMethod: "POST", Endpoint: "/upgrade", Func: n.Upgrade},
		{HTTPMethod: "GET", Endpoint: "/status", Func: n.Status},
		{HTTPMethod: "GET", Endpoint: "/configuration", Func: n.GetConfiguration},
		{HTTPMethod: "POST", Endpoint: "/configuration", Func: n.UpdateConfiguration}}
}

func (n *NginxAgent) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	io.Copy(&buf, r.Body)

	dataBytes := buf.Bytes()
	err := ioutil.WriteFile(confPath, dataBytes, 0644)

	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]string{"status:": "error", "message": err.Error()})
	} else {
		respondWithJSON(w, http.StatusOK, map[string]string{"status:": "ok"})
	}
}

func (n *NginxAgent) GetConfiguration(w http.ResponseWriter, r *http.Request) {
	config, _ := ioutil.ReadFile(confPath)
	respondWithJSON(w, http.StatusOK, string(config))
}

func (n *NginxAgent) Upgrade(w http.ResponseWriter, r *http.Request) {
	common.Log.Println("Node is upgrading")
	isUpgrading = true

	for _, cmd := range upgradeCommands {
		out, err := n.RunCustomCommand(cmd)
		common.Log.Println(out)
		if err != nil {
			isUpgrading = false
			common.Log.Println(err)
			respondWithJSON(w, http.StatusOK, map[string]string{"upgradeResult": "failed"})
			return
		}
	}

	isUpgrading = false
	common.Log.Println("Upgrade finished")
	respondWithJSON(w, http.StatusOK, map[string]string{"upgradeResult": "succeeded"})
}

func (n *NginxAgent) Status(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "running"})
}
