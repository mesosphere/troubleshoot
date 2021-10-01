package collect

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultCephNamespace = "rook-ceph"
)

type CephCommand struct {
	ID             string
	Command        []string
	Args           []string
	Format         string
	DefaultTimeout string
}

var CephCommands = []CephCommand{
	{
		ID:      "status",
		Command: []string{"ceph", "status"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "fs",
		Command: []string{"ceph", "fs", "status"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "fs-ls",
		Command: []string{"ceph", "fs", "ls"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "osd-status",
		Command: []string{"ceph", "osd", "status"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "txt",
	},
	{
		ID:      "osd-tree",
		Command: []string{"ceph", "osd", "tree"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "osd-pool",
		Command: []string{"ceph", "osd", "pool", "ls", "detail"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "health",
		Command: []string{"ceph", "health", "detail"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:      "auth",
		Command: []string{"ceph", "auth", "ls"},
		Args:    []string{"-f", "json-pretty"},
		Format:  "json",
	},
	{
		ID:             "rgw-stats", // the disk usage (and other stats) of each object store bucket
		Command:        []string{"radosgw-admin", "bucket", "stats"},
		Args:           []string{"--rgw-cache-enabled=false"},
		Format:         "txt",
		DefaultTimeout: "30s", // include a default timeout because this command will hang if the RGW daemon isn't running/is unhealthy
	},
	{
		ID:      "rbd-du", // the disk usage of each PVC
		Command: []string{"rbd", "du"},
		Args:    []string{"--pool=replicapool"},
		Format:  "txt",
	},
}

func Ceph(c *Collector, cephCollector *troubleshootv1beta2.Ceph) (CollectorResult, error) {
	ctx := context.TODO()

	if cephCollector.Namespace == "" {
		cephCollector.Namespace = DefaultCephNamespace
	}

	pod, err := findRookCephToolsPod(ctx, c, cephCollector.Namespace)
	if err != nil {
		return nil, err
	}

	output := NewResult()
	var multiErr *multierror.Error
	for _, command := range CephCommands {
		results, err := cephCommandExec(ctx, c, cephCollector, pod, command)
		multiErr = multierror.Append(multiErr, errors.Wrapf(err, "failed to exec command %s", command.ID))
		for fileName, r := range results {
			output.SaveResult(c.BundlePath, fileName, bytes.NewBuffer(r))
		}
	}
	return output, nil
}

func cephCommandExec(ctx context.Context, c *Collector, cephCollector *troubleshootv1beta2.Ceph, pod *corev1.Pod, command CephCommand) (map[string][]byte, error) {
	timeout := cephCollector.Timeout
	if timeout == "" {
		timeout = command.DefaultTimeout
	}

	execCollector := &troubleshootv1beta2.Exec{
		Selector:  labelsToSelector(pod.Labels),
		Namespace: pod.Namespace,
		Command:   command.Command,
		Args:      command.Args,
		Timeout:   timeout,
	}
	results, err := Exec(c, execCollector)
	if err != nil {
		return nil, err
	}

	final := map[string][]byte{}
	for filename, result := range results {
		pathPrefix := GetCephCollectorFilepath(cephCollector.CollectorName, cephCollector.Namespace)
		switch {
		case strings.HasSuffix(filename, "-stdout.txt"):
			final[path.Join(pathPrefix, fmt.Sprintf("%s.%s", command.ID, command.Format))] = result
		case strings.HasSuffix(filename, "-stderr.txt"):
			final[path.Join(pathPrefix, fmt.Sprintf("%s-stderr.txt", command.ID))] = result
		case strings.HasSuffix(filename, "-errors.json"):
			final[path.Join(pathPrefix, fmt.Sprintf("%s-errors.json", command.ID))] = result
		}
	}

	return final, nil
}

func findRookCephToolsPod(ctx context.Context, c *Collector, namespace string) (*corev1.Pod, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	pods, _ := listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-tools"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	pods, _ = listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-operator"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	return nil, errors.New("rook ceph tools pod not found")
}

func GetCephCollectorFilepath(name, namespace string) string {
	parts := []string{}
	if name != "" {
		parts = append(parts, name)
	}
	if namespace != "" && namespace != DefaultCephNamespace {
		parts = append(parts, namespace)
	}
	parts = append(parts, "ceph")
	return path.Join(parts...)
}

func labelsToSelector(labels map[string]string) []string {
	selector := []string{}
	for key, value := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", key, value))
	}
	return selector
}
