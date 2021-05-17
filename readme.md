# Forwarder

> A forwarder for forwarding k8s cluster pod programmatically.


## Installation

```bash
go get github.com/anthhub/forwarder
```


## Usage

```go
	options := []*Option{
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
			LocalPort: 8081,
			PodPort:   80,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment-66b6c48dd5-n86z4",
					Namespace: "default",
				},
			},
		},
	}

	// create a forwarder
	ret, err := WithForwarders(context.Background(), options, *kubecfg)
	if err != nil {
		panic(err)
	}
	// remember to close the forwarding
	defer ret.Close()
	// wait the ready of forwarding
	ret.Ready()
	// ...

	// if you want to block the goroutine and listen IOStreams close signal, you can do as following:
	// 
	// if err := ret.Wait(); err != nil {
	// 	panic(err)
	// }
```

> If you want to learn more about `forwarder`, you can read test cases and source code.
