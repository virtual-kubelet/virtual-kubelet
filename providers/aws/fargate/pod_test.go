package fargate

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

// TestTaskSizeTableInvariants verifies that the task size table is in ascending order by CPU.
// This is necessary for Pod::mapTaskSize to function correctly.
func TestTaskSizeTableInvariants(t *testing.T) {
	prevRow := taskSizeTable[0]

	for _, row := range taskSizeTable {
		assert.Check(t, row.cpu >= prevRow.cpu, "Task size table must be in ascending order by CPU")
		prevRow = row
	}
}

// TestPodResourceRequirements verifies whether Kubernetes pod resource requirements
// are translated to Fargate task resource requests correctly.
func TestPodResourceRequirements(t *testing.T) {
	type testCase struct {
		podCPU     int64
		podMemory  int64
		taskCPU    int64
		taskMemory int64
	}

	testCases := []testCase{
		{0, 0, 256, 512},
		{1, 1, 256, 512},
		{200, 256, 256, 512},
		{200, 512, 256, 512},
		{256, 3072, 512, 3072},

		{256, 512, 256, 512},
		{256, 1024, 256, 1024},
		{256, 2048, 256, 2048},

		{512, 1024, 512, 1024},
		{512, 2048, 512, 2048},
		{512, 3072, 512, 3072},
		{512, 4096, 512, 4096},

		{1024, 2 * 1024, 1024, 2 * 1024},
		{1024, 3 * 1024, 1024, 3 * 1024},
		{1024, 4 * 1024, 1024, 4 * 1024},
		{1024, 5 * 1024, 1024, 5 * 1024},
		{1024, 6 * 1024, 1024, 6 * 1024},
		{1024, 7 * 1024, 1024, 7 * 1024},
		{1024, 8 * 1024, 1024, 8 * 1024},

		{2048, 4 * 1024, 2048, 4 * 1024},
		{2048, 5 * 1024, 2048, 5 * 1024},
		{2048, 6 * 1024, 2048, 6 * 1024},
		{2048, 7 * 1024, 2048, 7 * 1024},
		{2048, 8 * 1024, 2048, 8 * 1024},
		{2048, 9 * 1024, 2048, 9 * 1024},
		{2048, 10 * 1024, 2048, 10 * 1024},
		{2048, 11 * 1024, 2048, 11 * 1024},
		{2048, 12 * 1024, 2048, 12 * 1024},
		{2048, 13 * 1024, 2048, 13 * 1024},
		{2048, 14 * 1024, 2048, 14 * 1024},
		{2048, 15 * 1024, 2048, 15 * 1024},
		{2048, 16 * 1024, 2048, 16 * 1024},

		{4096, 8 * 1024, 4096, 8 * 1024},
		{4096, 9 * 1024, 4096, 9 * 1024},
		{4096, 10 * 1024, 4096, 10 * 1024},
		{4096, 11 * 1024, 4096, 11 * 1024},
		{4096, 12 * 1024, 4096, 12 * 1024},
		{4096, 13 * 1024, 4096, 13 * 1024},
		{4096, 14 * 1024, 4096, 14 * 1024},
		{4096, 15 * 1024, 4096, 15 * 1024},
		{4096, 16 * 1024, 4096, 16 * 1024},
		{4096, 17 * 1024, 4096, 17 * 1024},
		{4096, 18 * 1024, 4096, 18 * 1024},
		{4096, 19 * 1024, 4096, 19 * 1024},
		{4096, 20 * 1024, 4096, 20 * 1024},
		{4096, 21 * 1024, 4096, 21 * 1024},
		{4096, 22 * 1024, 4096, 22 * 1024},
		{4096, 23 * 1024, 4096, 23 * 1024},
		{4096, 24 * 1024, 4096, 24 * 1024},
		{4096, 25 * 1024, 4096, 25 * 1024},
		{4096, 26 * 1024, 4096, 26 * 1024},
		{4096, 27 * 1024, 4096, 27 * 1024},
		{4096, 28 * 1024, 4096, 28 * 1024},
		{4096, 29 * 1024, 4096, 29 * 1024},
		{4096, 30 * 1024, 4096, 30 * 1024},

		{4097, 30 * 1024, 0, 0},
		{4096, 30*1024 + 1, 0, 0},
		{4096, 32 * 1024, 0, 0},
		{8192, 64 * 1024, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(
			fmt.Sprintf("cpu:%v,memory:%v", tc.podCPU, tc.podMemory),
			func(t *testing.T) {
				pod := &Pod{
					taskCPU:    tc.podCPU,
					taskMemory: tc.podMemory,
				}

				err := pod.mapTaskSize()
				if tc.taskCPU != 0 {
					// Test case is expected to succeed.
					assert.Check(t, err,
						"mapTaskSize failed for (cpu:%v memory:%v)",
						tc.podCPU, tc.podMemory)
					if err != nil {
						return
					}
				} else {
					// Test case is expected to fail.
					assert.Check(t, is.ErrorContains(err, ""), "mapTaskSize expected to fail but succeeded for (cpu:%v memory:%v)",
						tc.podCPU, tc.podMemory)
					return
				}

				assert.Check(t, pod.taskCPU >= tc.podCPU, "pod assigned less cpu than requested")
				assert.Check(t, pod.taskMemory >= tc.podMemory, "pod assigned less memory than requested")

				assert.Check(t,
					pod.taskCPU == tc.taskCPU && pod.taskMemory == tc.taskMemory,
					"requested (cpu:%v memory:%v) expected (cpu:%v memory:%v) observed (cpu:%v memory:%v)\n",
					tc.podCPU, tc.podMemory, tc.taskCPU, tc.taskMemory, pod.taskCPU, pod.taskMemory)
			})
	}
}
