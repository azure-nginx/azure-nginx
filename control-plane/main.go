package main

import (
	"flag"
	"fmt"

	"github.com/azure-nginx/azure-nginx/common"
)

var (
	logpath = flag.String("logpath", "/var/log/nginxcontrolplane/cp.log", "Log Path")
)

func main() {
	fmt.Println("control plane started")

	common.NewLog(*logpath)

	a := App{}
	a.Run()
}
