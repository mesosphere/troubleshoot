package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/metrics/pkg/apis/custom_metrics"
)

const (
	namespaceSingular = "namespace"
	namespacePlural   = "namespaces"
	urlBase           = "/apis/custom.metrics.k8s.io/v1beta1"
	metricsErrorFile  = "metrics/errors.json"
)

type CollectMetrics struct {
	Collector    *troubleshootv1beta2.CustomMetrics
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectMetrics) Title() string {
	return getCollectorName(c)
}

func (c *CollectMetrics) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectMetrics) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()
	resultLists := make(map[string][]custom_metrics.MetricValue)
	errorsList := make([]error, 0)
	for _, metricRequest := range c.Collector.MetricRequests {
		endpoint, metricName, err := constructEndpoint(metricRequest)
		if err != nil {
			errorsList = append(errorsList, errors.Wrapf(err, "could not construct endpoint for %s", metricRequest.ResourceMetricName))
			continue
		}
		response, err := c.Client.CoreV1().RESTClient().Get().AbsPath(endpoint).DoRaw(c.Context)
		if err != nil {
			errorsList = append(errorsList, errors.Wrapf(err, "could not query endpoint %s", endpoint))
			continue
		}
		metricsValues := custom_metrics.MetricValueList{}
		json.Unmarshal(response, &metricsValues)
		// metrics
		// |_ <resource_type>
		//    |_ <metric_name>
		//       |_ <namespace>.json or <non_namespaced_object>.json
		var path []string
		for _, item := range metricsValues.Items {
			item.Metric.Name = metricName
			if item.DescribedObject.Namespace != "" {
				path = []string{"metrics", item.DescribedObject.Kind, metricName, fmt.Sprintf("%s.json", item.DescribedObject.Namespace)}
			} else {
				path = []string{"metrics", item.DescribedObject.Kind, metricName, fmt.Sprintf("%s.json", item.DescribedObject.Name)}
			}
			filePath := filepath.Join(path...)
			if _, ok := resultLists[filePath]; !ok {
				resultLists[filePath] = make([]custom_metrics.MetricValue, 0)
			}
			resultLists[filePath] = append(resultLists[filePath], item)
		}
	}

	// Construct output.
	for relativePath, list := range resultLists {
		payload, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			errorsList = append(errorsList, errors.Wrapf(err, "could not format readings for %s", relativePath))
		}
		output.SaveResult(c.BundlePath, relativePath, bytes.NewBuffer(payload))
	}
	errPayload, err := json.MarshalIndent(errorsList, "", "  ")
	if err != nil {
		return nil, err
	}
	output.SaveResult(c.BundlePath, metricsErrorFile, bytes.NewBuffer(errPayload))
	return output, nil
}

func constructEndpoint(metricRequest troubleshootv1beta2.MetricRequest) (string, string, error) {
	metricNameComponents := strings.Split(metricRequest.ResourceMetricName, "/")
	if len(metricNameComponents) != 2 {
		return "", "", errors.New("wrong metric name format %s")
	}
	objectType := metricNameComponents[0]
	// Namespace related metrics are grouped under singular format "namespace/"
	// unlike other resources.
	if objectType == namespacePlural {
		objectType = namespaceSingular
	}
	metricName := metricNameComponents[1]
	objectSelector := "*"
	if metricRequest.ObjectName != "" {
		objectSelector = metricRequest.ObjectName
	}
	var endpoint string
	var err error
	if metricRequest.Namespace != "" {
		// namespaced objects
		// endpoint <resource_type>/namespaces/<namespace>/<resrouce_name or *>/<metric>
		endpoint, err = url.JoinPath(urlBase, namespacePlural, metricRequest.Namespace, objectType, objectSelector, metricName)
		if err != nil {
			return "", "", errors.Wrap(err, "could not construct url")
		}
	} else {
		// non-namespaced objects
		// endpoint <resource_type>/<resrouce_name or *>/<metric>
		endpoint, err = url.JoinPath(urlBase, objectType, objectSelector, metricName)
		if err != nil {
			return "", "", errors.Wrap(err, "could not construct url")
		}
	}
	return endpoint, metricName, nil
}
