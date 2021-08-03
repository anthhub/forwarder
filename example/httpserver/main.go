package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"

	"github.com/anthhub/forwarder"
	"github.com/gin-gonic/gin"

	"github.com/namsral/flag"
)

func setupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	return r
}

func main() {
	var kubecfg string
	flag.StringVar(&kubecfg, "kubeconfig", "../kubeconfig", `

	the path of kubeconfig, default is '~/.kube/config'
	you can configure kubeconfig by environment variable: KUBECONFIG=./kubeconfig, 
	or provide a option: --kubeconfig=./kubeconfig

	`)
	flag.Parse()
	fmt.Printf("kubecfg: %v", kubecfg)

	options := []*forwarder.Option{
		{
			LocalPort:   8080,
			RemotePort:  80,
			ServiceName: "my-nginx-svc",
		},
		{
			// LocalPort: 8081,
			// RemotePort:   80,
			Source: "po/my-nginx-66b6c48dd5-ttdb2",
		},
	}

	// it's to create a forwarder, and you need provide a path of kubeconfig
	ret, err := forwarder.WithForwarders(context.Background(), options, kubecfg)
	if err != nil {
		panic(err)
	}

	// remember to close the forwarding
	defer ret.Close()
	// wait forwarding ready
	// the remote and local ports are listed
	ports, err := ret.Ready()
	if err != nil {
		panic(err)
	}
	fmt.Printf("ports: %+v\n", ports)
	// ...

	// if you want to block the goroutine and listen IOStreams close signal, you can do as following:
	// ret.Wait()

	r := setupRouter()
	// Listen and Server in 0.0.0.0:8000
	r.Run(":8000")
}
