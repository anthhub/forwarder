# Forwarder

> A forwarder for forwarding k8s cluster pod programmatically.


## Installation

```bash
go get github.com/anthhub/forwarder
```


## Usage

```go

	import (
		"github.com/anthhub/forwarder"
	)

	options := []*forwarder.Option{
		{
			// the local port for forwarding
			LocalPort: 8080,
			// the k8s pod port
			PodPort:   80,
			// the k8s pod metadata
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment-66b6c48dd5-5pkwm",
					Namespace: "default",
				},
			},
		},
		{
			// if local port isn't provided, forwarder will generate a random port number
			// LocalPort: 8081,
			PodPort:   80,
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
	ret, err := forwarder.WithForwarders(context.Background(), options, "./kubecfg")
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
	// ...

	// if you want to block the goroutine and listen IOStreams close signal, you can do as following:
	ret.Wait()
```

> If you want to learn more about `forwarder`, you can read test cases and source code.
