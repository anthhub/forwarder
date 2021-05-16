package forwarder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type portForwardAPodRequest struct {
	// RestConfig is the kubernetes config
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod v1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// PodPort is the target port for the pod
	PodPort int
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

type signal struct {
	StopCh  chan struct{}
	ReadyCh chan struct{}
}

type Option struct {
	LocalPort int
	PodPort   int
	Pod       v1.Pod
}

type Result struct {
	Close func()
	Ready func()
	Wait  func() error
}

var once sync.Once

func WithForwarders(ctx context.Context, options []*Option, kubeconfig string) (*Result, error) {

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	var signals []*signal

	var g errgroup.Group

	for _, option := range options {
		stopCh := make(chan struct{}, 1)
		readyCh := make(chan struct{})

		signals = append(signals, &signal{stopCh, readyCh})

		req := &portForwardAPodRequest{
			RestConfig: config,
			Pod:        option.Pod,
			LocalPort:  option.LocalPort,
			PodPort:    option.PodPort,
			Streams:    stream,
			StopCh:     stopCh,
			ReadyCh:    readyCh,
		}

		g.Go(func() error {
			return withForwarder(req)
		})
	}

	ret := &Result{
		Close: func() {
			once.Do(func() {
				for _, s := range signals {
					close(s.StopCh)
				}
			})
		},
		Ready: func() {
			for _, s := range signals {
				<-s.ReadyCh
			}
		},
		Wait: g.Wait,
	}

	go func() {
		<-ctx.Done()
		ret.Close()
	}()

	return ret, nil
}

func withForwarder(req *portForwardAPodRequest) error {
	err := portForwardAPod(req)
	if err != nil {
		return err
	}
	return nil
}

func portForwardAPod(req *portForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}
