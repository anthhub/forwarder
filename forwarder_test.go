package forwarder

import (
	"context"
	"flag"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var kubecfg = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")

func TestBasic(t *testing.T) {
	flag.Parse()

	options := []*Option{
		{
			LocalPort: 8080,
			PodPort:   80,
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

	ret, err := WithForwarders(context.Background(), options, *kubecfg)
	if err != nil {
		panic(err)
	}
	defer ret.Close()
	ret.Ready()
	if err := ret.Wait(); err != nil {
		panic(err)
	}

}
