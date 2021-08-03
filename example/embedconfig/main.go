package main

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/anthhub/forwarder"
)

//go:embed kubeconfig
var kubeconfigBytes []byte

func main() {
	options := []*forwarder.Option{
		{
			// LocalPort: 8080,
			// RemotePort:  80,
			ServiceName: "my-nginx-svc",
		},
	}

	ret, err := forwarder.WithForwardersEmbedConfig(context.Background(), options, kubeconfigBytes)
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
	ret.Wait()
}
