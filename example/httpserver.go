package main

import (
	"context"

	"fmt"
	"net/http"

	"github.com/anthhub/forwarder"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/namsral/flag"
	v1 "k8s.io/api/core/v1"
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
			// the local port for forwarding
			LocalPort: 8080,
			// the k8s pod port
			PodPort: 80,
			// the k8s pod metadata
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-nginx-66b6c48dd5-ttdb2",
					Namespace: "default",
				},
			},
		},
		{
			// if local port isn't provided, forwarder will generate a random port number
			// LocalPort: 8081,
			PodPort: 80,
			// the k8s service metadata, it's to forward service
			Service: v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-nginx-svc",
					// the namespace default is "default"
					// Namespace: "default",
				},
			},
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
