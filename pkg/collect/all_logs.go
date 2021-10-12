package collect

import (
	"context"
	"fmt"
	"strings"

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
	// If Namespaces list is empty, get all the namespaces
	if len(logsCollector.Namespaces) == 0 || logsCollector.Namespaces[0] == "*" {
		_, namespaceList, namespaceErrors := getAllNamespaces(ctx, client)
		if len(namespaceErrors) > 0 {
			output.SaveResult(c.BundlePath, getAllLogsErrorsFileName(logsCollector), marshalErrors(namespaceErrors))
		}
		logsCollector.Namespaces = getNamespaceNames(namespaceList)
	}

	for _, namespace := range logsCollector.Namespaces {
		pods, podsErrors := listPodsInNamespace(ctx, client, namespace, logsCollector.Selector)
		if len(podsErrors) > 0 {
			output.SaveResult(c.BundlePath, getAllLogsErrorsFileName(logsCollector), marshalErrors(podsErrors))
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
					podLogs, err := savePodLogs(ctx, c.BundlePath, client, pod, logsCollector.Name, containerName, logsCollector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s-errors.json", logsCollector.Name, pod.Name)
						if containerName != "" {
							key = fmt.Sprintf("%s/%s/%s-errors.json", logsCollector.Name, pod.Name, containerName)
						}
						output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
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
					containerLogs, err := savePodLogs(ctx, c.BundlePath, client, pod, logsCollector.Name, container, logsCollector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s/%s-errors.json", logsCollector.Name, pod.Name, container)
						output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
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
	// TODO: random part
	return "errors.json"
}
