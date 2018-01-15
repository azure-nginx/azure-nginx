package main

import (
	"os"
)

func main() {
	a := App{}
	a.Run(os.Getenv("HTTP_PORT"), os.Getenv("APP_ID"), os.Getenv("CLIENT_SECRET"), os.Getenv("TENANT_ID"), os.Getenv("SUBSCRIPTION_ID"))
}
