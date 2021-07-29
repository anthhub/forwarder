package forwarder

import (
	"context"

	"fmt"
	"testing"

	"github.com/namsral/flag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBasic(t *testing.T) {
	var kubecfg string
	flag.StringVar(&kubecfg, "kubeconfig", "./kubeconfig", `

	the path of kubeconfig, default is '~/.kube/config'
	you can configure kubeconfig by environment variable: KUBECONFIG=./kubeconfig, 
	or provide a option: --kubeconfig=./kubeconfig

	`)
	flag.Parse()
	fmt.Printf("kubecfg: %v", kubecfg)

	options := []*Option{
		{
			// LocalPort: 8080,
			PodPort: 80,
			Service: v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-nginx-svc",
					// Namespace: "default",
				},
			},
		},
		{
			LocalPort: 8081,
			PodPort:   80,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-nginx-66b6c48dd5-ttdb2",
					// Namespace: "default",
				},
			},
		},
	}

	ret, err := WithForwarders(context.Background(), options, kubecfg)
	if err != nil {
		panic(err)
	}
	defer ret.Close()
	ports, err := ret.Ready()
	if err != nil {
		panic(err)
	}
	fmt.Printf("ports: %+v\n", ports)
	ret.Wait()

}
