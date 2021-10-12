package collect

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func AllLogs(c *Collector, logsCollector *troubleshootv1beta2.AllLogs) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	// Set default logCollector Name if empty
	logCollectorName := "allPodLogs"

	if logsCollector.Name != "" {
		logCollectorName = logsCollector.Name
	}

	// If Namespaces list is empty, get all the namespaces
	if len(logsCollector.Namespaces) == 0 || logsCollector.Namespaces[0] == "*" {
		_, namespaceList, namespaceErrors := getAllNamespaces(ctx, client)
		if len(namespaceErrors) > 0 {
			err = output.SaveResult(c.BundlePath, getAllLogsErrorsFileName(logsCollector), marshalErrors(namespaceErrors))
			if err != nil {
				return output, err
			}
		}
		logsCollector.Namespaces = getNamespaceNames(namespaceList)
	}

	for _, namespace := range logsCollector.Namespaces {
		pods, podsErrors := listPodsInNamespace(ctx, client, namespace, logsCollector.Selector)
		if len(podsErrors) > 0 {
			err = output.SaveResult(c.BundlePath, getAllLogsErrorsFileName(logsCollector), marshalErrors(podsErrors))
			if err != nil {
				return nil, err
			}
		}

		for _, pod := range pods {
			if len(logsCollector.ContainerNames) == 0 {
				// make a list of all the containers in the pod, so that we can get logs from all of them
				containerNames := []string{}
				for _, container := range pod.Spec.Containers {
					containerNames = append(containerNames, container.Name)
				}
				for _, container := range pod.Spec.InitContainers {
					containerNames = append(containerNames, container.Name)
				}

				for _, containerName := range containerNames {
					if len(containerNames) == 1 {
						containerName = "" // if there was only one container, use the old behavior of not including the container name in the path
					}
					podLogs, err := saveNamespacedPodLogs(ctx, c.BundlePath, client, pod, logCollectorName, containerName, namespace, logsCollector.Limits)
					if err != nil {
						key := fmt.Sprintf("%s/%s/%s-errors.json", logCollectorName, namespace, pod.Name)
						if containerName != "" {
							key = fmt.Sprintf("%s/%s/%s/%s-errors.json", logCollectorName, namespace, pod.Name, containerName)
						}
						err = output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range podLogs {
						output[k] = v
					}
				}
			} else {
				for _, container := range logsCollector.ContainerNames {
					containerLogs, err := saveNamespacedPodLogs(ctx, c.BundlePath, client, pod, logCollectorName, container, namespace, logsCollector.Limits)
					if err != nil {
						key := fmt.Sprintf("%s/%s/%s/%s-errors.json", logCollectorName, namespace, pod.Name, container)
						err = output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range containerLogs {
						output[k] = v
					}
				}
			}
		}
	}

	return output, nil
}

func listPodsInNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.Pod, []error) {
	var (
		pods        *corev1.PodList
		err         error
		listOptions metav1.ListOptions
	)
	// Providing selectors is optional for AllPods collector.
	if len(selector) != 0 {
		serializedLabelSelector := strings.Join(selector, ",")

		listOptions.LabelSelector = serializedLabelSelector
	}

	pods, err = client.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, []error{err}
	}

	return pods.Items, nil
}

func getNamespaceNames(namespaceList *corev1.NamespaceList) []string {
	var namespaceNames []string

	if namespaceList == nil {
		return namespaceNames
	}

	for _, namespace := range namespaceList.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}
	return namespaceNames
}

func getAllLogsErrorsFileName(logsCollector *troubleshootv1beta2.AllLogs) string {
	if len(logsCollector.Name) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.Name)
	} else if len(logsCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.CollectorName)
	}

	return "errors.json"
}

func saveNamespacedPodLogs(ctx context.Context, bundlePath string, client *kubernetes.Clientset, pod corev1.Pod, name, container, namespace string, limits *troubleshootv1beta2.LogLimits) (CollectorResult, error) {
	podLogOpts := corev1.PodLogOptions{
		Follow:    false,
		Container: container,
	}

	setLogLimits(&podLogOpts, limits, convertMaxAgeToTime)

	fileKey := fmt.Sprintf("%s/%s/%s", name, namespace, pod.Name)
	if container != "" {
		fileKey = fmt.Sprintf("%s/%s/%s/%s", name, namespace, pod.Name, container)
	}

	result := NewResult()

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer podLogs.Close()

	logWriter, err := result.GetWriter(bundlePath, fileKey+".log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log writer")
	}
	defer result.CloseWriter(bundlePath, fileKey+".log", logWriter)

	_, err = io.Copy(logWriter, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy log")
	}

	podLogOpts.Previous = true
	req = client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err = req.Stream(ctx)
	if err != nil {
		// maybe fail on !kuberneteserrors.IsNotFound(err)?
		return result, nil
	}
	defer podLogs.Close()

	prevLogWriter, err := result.GetWriter(bundlePath, fileKey+"-previous.log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get previous log writer")
	}
	defer result.CloseWriter(bundlePath, fileKey+"-previous.log", logWriter)

	_, err = io.Copy(prevLogWriter, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy previous log")
	}

	return result, nil
}
