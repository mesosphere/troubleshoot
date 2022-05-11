package collect

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/shirou/gopsutil/mem"
)

type MemoryInfo struct {
	Total uint64 `json:"total"`
}

type CollectHostMemory struct {
	hostCollector *troubleshootv1beta2.Memory
	BundlePath    string
}

func (c *CollectHostMemory) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Amount of Memory")
}

func (c *CollectHostMemory) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostMemory) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	memoryInfo := MemoryInfo{}

	vmstat, err := mem.VirtualMemory()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read virtual memory")
	}
	memoryInfo.Total = vmstat.Total

	b, err := json.Marshal(memoryInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal memory info")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "memory"
	}
	name := filepath.Join("system", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}
