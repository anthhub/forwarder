package forwarder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
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

type carry struct {
	StopCh  chan struct{}
	ReadyCh chan struct{}
	PF      *portforward.PortForwarder
}

type Option struct {
	LocalPort int        // the local port for forwarding
	PodPort   int        // the k8s pod port
	Pod       v1.Pod     // the k8s pod metadata
	Service   v1.Service // the k8s service metadata
}

type Result struct {
	Close func()                                        // close the port forwarding
	Ready func() ([][]portforward.ForwardedPort, error) // block till the forwarding ready
	Wait  func()                                        // block and listen IOStreams close signal
}

// It is to forward port for k8s cloud services.
func WithForwarders(ctx context.Context, options []*Option, kubeconfig string) (*Result, error) {
	if kubeconfig == "" {
		kubeconfig = "./kubeconfig"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	newOptions, err := handleOptions(ctx, options, config)
	if err != nil {
		return nil, err
	}

	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	carries := make([]*carry, len(newOptions))

	var g errgroup.Group

	for index, option := range newOptions {
		index := index
		stopCh := make(chan struct{}, 1)
		readyCh := make(chan struct{})

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
			pf, err := portForwardAPod(req)
			if err != nil {
				return err
			}
			carries[index] = &carry{StopCh: stopCh, ReadyCh: readyCh, PF: pf}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	ret := &Result{
		Close: func() {
			once.Do(func() {
				for _, c := range carries {
					close(c.StopCh)
				}
			})
		},
		Ready: func() ([][]portforward.ForwardedPort, error) {
			pfs := [][]portforward.ForwardedPort{}
			for _, c := range carries {
				<-c.ReadyCh
				ports, err := c.PF.GetPorts()
				if err != nil {
					return nil, err
				}
				pfs = append(pfs, ports)
			}
			return pfs, nil
		},
	}

	ret.Wait = func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		fmt.Println("Bye...")
		ret.Close()
	}

	go func() {
		<-ctx.Done()
		ret.Close()
	}()

	return ret, nil
}

func portForwardAPod(req *portForwardAPodRequest) (*portforward.PortForwarder, error) {
	namespace := req.Pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := fw.ForwardPorts(); err != nil {
			panic(err)
		}
	}()

	return fw, nil
}

func handleOptions(ctx context.Context, options []*Option, config *restclient.Config) ([]*Option, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	newOptions := make([]*Option, len(options))
	var g errgroup.Group

	for index, option := range options {
		option := option
		index := index

		g.Go(func() error {
			podName := option.Pod.ObjectMeta.Name

			if podName != "" {
				namespace := option.Pod.ObjectMeta.Namespace
				if namespace == "" {
					namespace = "default"
				}
				pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if pod == nil {
					return fmt.Errorf("no such pod: %v", podName)
				}

				newOptions[index] = option
				return nil
			}

			svcName := option.Service.ObjectMeta.Name
			if svcName == "" {
				return fmt.Errorf("please provide a pod or service")
			}
			namespace := option.Service.ObjectMeta.Namespace
			if namespace == "" {
				namespace = "default"
			}

			svc, err := clientset.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if svc == nil {
				return fmt.Errorf("no such service: %v", svcName)
			}
			labels := []string{}
			for key, val := range svc.Spec.Selector {
				labels = append(labels, key+"="+val)
			}
			label := strings.Join(labels, ",")

			pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label, Limit: 1})
			if err != nil {
				return err
			}
			if len(pods.Items) == 0 {
				return fmt.Errorf("no such pods of the service of %v", svcName)
			}
			pod := pods.Items[0]

			fmt.Printf("\nForwarding service: %v to pod %v ...\n\n", svcName, pod.Name)

			newOptions[index] = &Option{
				LocalPort: option.LocalPort,
				PodPort:   option.PodPort,
				Pod: v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
				},
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return newOptions, nil
}
