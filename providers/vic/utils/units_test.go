package utils

import (
	"testing"

	"strings"

	"gotest.tools/assert"
)

func TestMemoryConversion(t *testing.T) {
	memSize := MemsizeToBinaryString(2, "Gb")
	assert.Assert(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2, "Gb")
	assert.Assert(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(2048, "Mb")
	assert.Assert(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2048, "Mb")
	assert.Assert(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(2048*1024, "Kb")
	assert.Assert(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2048*1024, "Kb")
	assert.Assert(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(MEMORYCUTOVER, "Gb")
	assert.Assert(t, memSize == "100Gi")

	memSize = MemsizeToBinaryString(MEMORYCUTOVER-1, "Gb")
	strings.HasSuffix(memSize, "Mi")
	assert.Assert(t, strings.HasSuffix(memSize, "Mi"))

	memSize = MemsizeToBinaryString((MEMORYCUTOVER-1)*1024, "Mb")
	assert.Assert(t, strings.HasSuffix(memSize, "Mi"))

	memSize = MemsizeToDecimalString(MEMORYCUTOVER*1000000000, "b")
	assert.Assert(t, memSize == "100G")

	memSize = MemsizeToDecimalString((MEMORYCUTOVER-1)*1000000000, "b")
	assert.Assert(t, strings.HasSuffix(memSize, "M"))
}

func TestFrequencyConversion(t *testing.T) {
	feq := CpuFrequencyToString(FREQUENCYCUTOVER, "Ghz")
	assert.Assert(t, feq == "10G")
	feq = CpuFrequencyToString(FREQUENCYCUTOVER-1, "Ghz")
	assert.Assert(t, feq == "9000M")
	feq = CpuFrequencyToString((FREQUENCYCUTOVER-1)*1000, "Mhz")
	assert.Assert(t, feq == "9000M")
}
