package main

import (
	"flag"
	"fmt"

	"github.com/azure-nginx/azure-nginx/common"
)

var (
	logpath = flag.String("logpath", "/var/log/nginxagent/agent.log", "Log Path")
)

func main() {
	fmt.Println("agent started")

	common.NewLog(*logpath)

	a := App{}
	a.Run()
}
