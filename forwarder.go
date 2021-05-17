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

var once sync.Once

type portForwardAPodRequest struct {
	RestConfig *rest.Config                // RestConfig is the kubernetes config
	Pod        v1.Pod                      // Pod is the selected pod for this port forwarding
	LocalPort  int                         // LocalPort is the local port that will be selected to expose the PodPort
	PodPort    int                         // PodPort is the target port for the pod
	Streams    genericclioptions.IOStreams // Steams configures where to write or read input from
	StopCh     <-chan struct{}             // StopCh is the channel used to manage the port forward lifecycle
	ReadyCh    chan struct{}               // ReadyCh communicates when the tunnel is ready to receive traffic
}

type signal struct {
	StopCh  chan struct{}
	ReadyCh chan struct{}
}

type Option struct {
	LocalPort int    // the local port for forwarding
	PodPort   int    // the k8s pod port
	Pod       v1.Pod // the k8s pod metadata
}

type Result struct {
	Close func()       // close the port forwarding
	Ready func()       // block till the forwarding ready
	Wait  func() error // block and listen IOStreams close signal
}

// It is to forward port for k8s cloud services.
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
