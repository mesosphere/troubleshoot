package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
	coreout "github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type UsageReading struct {
	Timestamp time.Time `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
	CPU       float64   `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory    float64   `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type Utilisation struct {
	Readings map[string][]UsageReading `json:"readings,omitempty" yaml:"readings,omitempty"`
}

type CollectCPUMemUtilisation struct {
	Collector    *troubleshootv1beta2.CPUMemUsage
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectCPUMemUtilisation) Title() string {
	return getCollectorName(c)
}

func (c *CollectCPUMemUtilisation) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

// todo: Rework.
func (c *CollectCPUMemUtilisation) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	thanosServiceNamespace := "kommander"
	output := NewResult()

	// Get the pods of the deployment
	pods, err := c.Client.CoreV1().Pods(thanosServiceNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=query,app.kubernetes.io/instance=thanos,app.kubernetes.io/name=thanos",
	})
	if err != nil {
		return nil, errors.Wrap(err, "Could not look up thanos-query pods: ")
	}
	if len(pods.Items) < 1 {
		// Check if no thanos-query pods are found.
		return nil, errors.New("Could not find any thanos-query pods.")
	}
	podName := pods.Items[0].Name

	// Create a Prometheus API client using the API server endpoint
	prometheusClient, stopCh, err := StartPortForwarding(context.Background(), c.ClientConfig, coreout.NewNonInteractiveShell(os.Stdout, os.Stderr, 4), podName)
	if err != nil {
		return nil, errors.Wrap(err, "Error querying Prometheus for CPU utilization:")
	}
	v1api := v1.NewAPI(prometheusClient)

	// Query for CPU utilization
	cpuQuery := "sum(rate(container_cpu_usage_seconds_total{container!='POD', container!=''}[1h])) by (namespace)"
	start := time.Now().Add(-1 * time.Duration(c.Collector.Duration) * 24 * time.Hour)
	end := time.Now()
	step := time.Duration(c.Collector.SamplingStep) * time.Hour
	cpuResult, _, err := v1api.QueryRange(context.Background(), cpuQuery, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error querying Prometheus for CPU utilization: ")
	}

	memoryQuery := "sum(container_memory_working_set_bytes{container!='POD', container!=''}) by (namespace)"
	memoryResult, _, err := v1api.QueryRange(context.Background(), memoryQuery, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Error querying Prometheus for memory utilization: ")
	}
	// Process query result
	memMatrix, ok := memoryResult.(model.Matrix)
	if !ok {
		return nil, errors.New("Invalid query result, expected matrix")
	}

	// Process query result
	cpuMatrix, ok := cpuResult.(model.Matrix)
	if !ok {
		return nil, errors.New("Invalid query result, expected matrix")
	}
	metricsByNamespace := make(map[string]map[string][]model.SamplePair, 0)
	for _, entry := range memMatrix {
		namespaceName := string(entry.Metric["namespace"])
		metricsByNamespace[namespaceName] = make(map[string][]model.SamplePair, 0)
		metricsByNamespace[namespaceName]["memory"] = entry.Values
		metricsByNamespace[namespaceName]["cpu"] = []model.SamplePair{}
	}
	for _, entry := range cpuMatrix {
		namespaceName := string(entry.Metric["namespace"])
		_, ok := metricsByNamespace[namespaceName]
		if ok {
			metricsByNamespace[namespaceName]["cpu"] = entry.Values
		} else {
			metricsByNamespace[namespaceName] = make(map[string][]model.SamplePair, 0)
			metricsByNamespace[namespaceName]["cpu"] = entry.Values
			metricsByNamespace[namespaceName]["memory"] = []model.SamplePair{}
		}
	}

	// Construct output.
	for namespaceName, readingsLists := range metricsByNamespace {
		jointReadings, err := joinReadings(readingsLists["cpu"], readingsLists["memory"])
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse readings: ")
		}

		payload, err := json.MarshalIndent(jointReadings, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "Error formatting readings: ")
		}
		path := []string{"utilisation", namespaceName}
		output.SaveResult(c.BundlePath, filepath.Join(path...), bytes.NewBuffer(payload))
	}
	stopCh <- struct{}{}
	return output, nil
}

func StartPortForwarding(
	ctx context.Context,
	restConfig *rest.Config,
	out output.Output,
	podName string,
) (client prom.Client, stopCh chan struct{}, err error) {
	var (
		localPort int
		lastErr   error
	)

	if err = wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		localPort, stopCh, lastErr = PortForward(
			restConfig, "kommander", podName, 10902, out,
		)
		return lastErr == nil, nil
	}); err != nil {
		return nil, nil, fmt.Errorf("%v: %w", err, lastErr)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", localPort)
	prometheusClient, err := prom.NewClient(prom.Config{
		Address: baseURL, RoundTripper: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return prometheusClient, stopCh, err
}

type PortForwardFn = func(*rest.Config, string, string, int, output.Output) (int, chan struct{}, error)

// PortForward creates an asynchronous port-forward to a pod on the cluster in order to
// access it over the local network.
func PortForward(cfg *rest.Config, ns, pod string, port int, out output.Output) (localPort int, stop chan struct{}, err error) {
	dialer, err := getDialer(cfg, ns, pod)
	if err != nil {
		return 0, nil, err
	}

	// configure the portforwarder
	stop, ready := make(chan struct{}, 1), make(chan struct{}, 1)
	pf, localPort, err := getPortForwarder(dialer, port, stop, ready, out)
	if err != nil {
		return localPort, nil, err
	}

	if err := runPortForwarder(pf, ready, out); err != nil {
		return -1, nil, err
	}

	return localPort, stop, nil
}

func getDialer(cfg *rest.Config, ns, pod string) (httpstream.Dialer, error) {
	roundTripper, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, err
	}

	serverURL, err := constructPortForwardURL(cfg.Host, ns, pod)
	if err != nil {
		return nil, err
	}

	return spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, serverURL), nil
}

func getPortForwarder(dialer httpstream.Dialer,
	port int, stop, ready chan struct{}, out output.Output,
) (*portforward.PortForwarder, int, error) {
	// get an available local port to use
	localPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, localPort, err
	}

	// configure the portforwarder
	pf, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, port)}, stop, ready, out.InfoWriter(), out.ErrorWriter())

	return pf, localPort, err
}

func runPortForwarder(pf *portforward.PortForwarder, ready chan struct{}, out output.Output) error {
	out.Info("running the port-forwarder in the background")
	errCh := make(chan error)
	// start the portforwarder in the background and look for errors
	go func() {
		if err := pf.ForwardPorts(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()
	// wait for the port-forward to be established and check for errors
	out.Info("waiting for port-forward to be established")
	select {
	case <-ready:
		out.Info("the port-forward has signaled ready")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("port-forward failed: %w", err)
		}
	}
	return nil
}

func constructPortForwardURL(host, ns, pod string) (*url.URL, error) {
	hostURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme: "https",
		Path: path.Join(
			hostURL.Path,
			"api/v1/namespaces",
			url.PathEscape(ns),
			"pods",
			url.PathEscape(pod),
			"portforward",
		),
		Host: hostURL.Host,
	}, nil
}

func joinReadings(cpuSamples, memSamples []model.SamplePair) ([]UsageReading, error) {
	jointReadingsList := make([]UsageReading, 0)
	i, j := 0, 0

	for i < len(cpuSamples) && j < len(memSamples) {
		if cpuSamples[i].Timestamp.Before(memSamples[j].Timestamp) {
			// Include item from CPU readings
			value, err := strconv.ParseFloat(cpuSamples[i].Value.String(), 64)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to parse float:")
			}
			jointReadingsList = append(jointReadingsList, UsageReading{
				Timestamp: cpuSamples[i].Timestamp.Time().UTC(),
				CPU:       value,
			})
			i++
		} else if cpuSamples[i].Timestamp.After(memSamples[j].Timestamp) {
			// Include item from memory readings
			value, err := strconv.ParseFloat(memSamples[j].Value.String(), 64)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to parse float:")
			}
			jointReadingsList = append(jointReadingsList, UsageReading{
				Timestamp: memSamples[j].Timestamp.Time().UTC(),
				Memory:    value,
			})
			j++
		} else {
			// Include items from both readings with matching timestamp
			cpu, err := strconv.ParseFloat(cpuSamples[i].Value.String(), 64)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to parse float:")
			}
			mem, err := strconv.ParseFloat(memSamples[j].Value.String(), 64)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to parse float:")
			}
			jointReadingsList = append(jointReadingsList, UsageReading{
				Timestamp: cpuSamples[i].Timestamp.Time().UTC(),
				CPU:       cpu,
				Memory:    mem,
			})
			i++
			j++
		}
	}

	// Include remaining items from cpu readings
	for i < len(cpuSamples) {
		cpu, err := strconv.ParseFloat(cpuSamples[i].Value.String(), 64)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse float:")
		}
		jointReadingsList = append(jointReadingsList, UsageReading{
			Timestamp: cpuSamples[i].Timestamp.Time().UTC(),
			CPU:       cpu,
		})
		i++
	}

	// Include remaining items from memory readings
	for j < len(memSamples) {
		mem, err := strconv.ParseFloat(memSamples[j].Value.String(), 64)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse float:")
		}
		jointReadingsList = append(jointReadingsList, UsageReading{
			Timestamp: memSamples[j].Timestamp.Time().UTC(),
			Memory:    mem,
		})
		j++
	}

	return jointReadingsList, nil
}
