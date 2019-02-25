package azure

import (
	"path"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/virtual-kubelet/azure-aci/client/aci"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func TestCollectMetrics(t *testing.T) {
	cases := []metricTestCase{
		{desc: "no containers"}, // this is just for sort of fuzzing things, make sure there's no panics
		{desc: "zeroed stats", stats: [][2]float64{{0, 0}}, rx: 0, tx: 0, collected: time.Now()},
		{desc: "normal", stats: [][2]float64{{400.0, 1000.0}}, rx: 100.0, tx: 5000.0, collected: time.Now()},
		{desc: "multiple containers", stats: [][2]float64{{100.0, 250.0}, {400.0, 1000.0}, {103.0, 3992.0}}, rx: 100.0, tx: 439833.0, collected: time.Now()},
	}

	for _, test := range cases {
		t.Run(test.desc, func(t *testing.T) {
			pod := fakePod(t, len(test.stats), time.Now())
			expected := podStatFromTestCase(t, pod, test)

			system, net := fakeACIMetrics(pod, test)
			actual := collectMetrics(pod, system, net)

			if len(actual.Containers) != len(expected.Containers) {
				t.Fatalf("got unexpected results\nexpected:\n%+v\nactual:\n%+v", expected, actual)
			}

			for _, actualContainer := range actual.Containers {
				found := false
				for _, expectedContainer := range expected.Containers {
					if expectedContainer.Name == actualContainer.Name {
						if !reflect.DeepEqual(expectedContainer, actualContainer) {
							t.Fatalf("got unexpected container\nexpected:\n%+v\nactual:\n%+v", expectedContainer, actualContainer)
						}
						found = true
						break
					}
				}

				if !found {
					t.Fatalf("Unexpected container:\n%+v", actualContainer)
				}
			}

			expected.Containers = nil
			actual.Containers = nil

			if !reflect.DeepEqual(expected, actual) {
				t.Fatalf("got unexpected results\nexpected:\n%+v\nactual:\n%+v", expected, actual)
			}
		})
	}
}

type metricTestCase struct {
	desc      string
	stats     [][2]float64
	rx, tx    float64
	collected time.Time
}

func fakeACIMetrics(pod *v1.Pod, testCase metricTestCase) (*aci.ContainerGroupMetricsResult, *aci.ContainerGroupMetricsResult) {
	newMetricValue := func(mt aci.MetricType) aci.MetricValue {
		return aci.MetricValue{
			Desc: aci.MetricDescriptor{
				Value: mt,
			},
		}
	}

	newNetMetric := func(collected time.Time, value float64) aci.MetricTimeSeries {
		return aci.MetricTimeSeries{
			Data: []aci.TimeSeriesEntry{
				{Timestamp: collected, Average: value},
			},
		}
	}

	newSystemMetric := func(c v1.ContainerStatus, collected time.Time, value float64) aci.MetricTimeSeries {
		return aci.MetricTimeSeries{
			Data: []aci.TimeSeriesEntry{
				{Timestamp: collected, Average: value},
			},
			MetadataValues: []aci.MetricMetadataValue{
				{Name: aci.ValueDescriptor{Value: "containerName"}, Value: c.Name},
			},
		}
	}

	// create fake aci metrics for the container group and test data
	cpuV := newMetricValue(aci.MetricTypeCPUUsage)
	memV := newMetricValue(aci.MetricTypeMemoryUsage)

	for i, c := range pod.Status.ContainerStatuses {
		cpuV.Timeseries = append(cpuV.Timeseries, newSystemMetric(c, testCase.collected, testCase.stats[i][0]))
		memV.Timeseries = append(memV.Timeseries, newSystemMetric(c, testCase.collected, testCase.stats[i][1]))
	}
	system := &aci.ContainerGroupMetricsResult{
		Value: []aci.MetricValue{cpuV, memV},
	}

	rxV := newMetricValue(aci.MetricTyperNetworkBytesRecievedPerSecond)
	txV := newMetricValue(aci.MetricTyperNetworkBytesTransmittedPerSecond)
	rxV.Timeseries = append(rxV.Timeseries, newNetMetric(testCase.collected, testCase.rx))
	txV.Timeseries = append(txV.Timeseries, newNetMetric(testCase.collected, testCase.tx))
	net := &aci.ContainerGroupMetricsResult{
		Value: []aci.MetricValue{rxV, txV},
	}
	return system, net
}

func fakePod(t *testing.T, size int, created time.Time) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              path.Base(t.Name()),
			Namespace:         path.Dir(t.Name()),
			UID:               types.UID(t.Name()),
			CreationTimestamp: metav1.NewTime(created),
		},
		Status: v1.PodStatus{
			Phase:             v1.PodRunning,
			ContainerStatuses: make([]v1.ContainerStatus, 0, size),
		},
	}

	for i := 0; i < size; i++ {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
			Name: "c" + strconv.Itoa(i),
		})
	}
	return pod
}

func podStatFromTestCase(t *testing.T, pod *v1.Pod, test metricTestCase) stats.PodStats {
	rx := uint64(test.rx)
	tx := uint64(test.tx)
	expected := stats.PodStats{
		StartTime: pod.CreationTimestamp,
		PodRef: stats.PodReference{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			UID:       string(pod.UID),
		},
		Network: &stats.NetworkStats{
			Time: metav1.NewTime(test.collected),
			InterfaceStats: stats.InterfaceStats{
				Name:    "eth0",
				RxBytes: &rx,
				TxBytes: &tx,
			},
		},
	}

	var (
		nodeCPU uint64
		nodeMem uint64
	)
	for i := range test.stats {
		cpu := uint64(test.stats[i][0] * 1000000)
		cpuNanoSeconds := cpu * 60
		mem := uint64(test.stats[i][1])

		expected.Containers = append(expected.Containers, stats.ContainerStats{
			StartTime: pod.CreationTimestamp,
			Name:      pod.Status.ContainerStatuses[i].Name,
			CPU:       &stats.CPUStats{Time: metav1.NewTime(test.collected), UsageNanoCores: &cpu, UsageCoreNanoSeconds: &cpuNanoSeconds},
			Memory:    &stats.MemoryStats{Time: metav1.NewTime(test.collected), UsageBytes: &mem, WorkingSetBytes: &mem},
		})
		nodeCPU += cpu
		nodeMem += mem
	}
	if len(expected.Containers) > 0 {
		nanoCPUSeconds := nodeCPU * 60
		expected.CPU = &stats.CPUStats{UsageNanoCores: &nodeCPU, UsageCoreNanoSeconds: &nanoCPUSeconds, Time: metav1.NewTime(test.collected)}
		expected.Memory = &stats.MemoryStats{UsageBytes: &nodeMem, WorkingSetBytes: &nodeMem, Time: metav1.NewTime(test.collected)}
	}
	return expected
}
