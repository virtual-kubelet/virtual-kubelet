package utils

import (
	"testing"

    "github.com/stretchr/testify/require"
	"strings"
)

func TestMemoryConversion(t *testing.T) {
	memSize := MemsizeToBinaryString(2, "Gb")
	require.True(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2, "Gb")
	require.True(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(2048, "Mb")
	require.True(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2048, "Mb")
	require.True(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(2048*1024, "Kb")
	require.True(t, memSize == "2048Mi")

	memSize = MemsizeToDecimalString(2048*1024, "Kb")
	require.True(t, memSize == "2147M")

	memSize = MemsizeToBinaryString(MEMORYCUTOVER, "Gb")
	require.True(t, memSize == "100Gi")

	memSize = MemsizeToBinaryString(MEMORYCUTOVER-1, "Gb")
	strings.HasSuffix(memSize, "Mi")
	require.True(t, strings.HasSuffix(memSize, "Mi"))

	memSize = MemsizeToBinaryString((MEMORYCUTOVER-1)*1024, "Mb")
	require.True(t, strings.HasSuffix(memSize, "Mi"))

	memSize = MemsizeToDecimalString(MEMORYCUTOVER*1000000000, "b")
	require.True(t, memSize == "100G")

	memSize = MemsizeToDecimalString((MEMORYCUTOVER-1)*1000000000, "b")
	require.True(t, strings.HasSuffix(memSize, "M"))
}

func TestFrequencyConversion(t *testing.T) {
	feq := CpuFrequencyToString(FREQUENCYCUTOVER, "Ghz")
	require.True(t, feq == "10G")
	feq = CpuFrequencyToString(FREQUENCYCUTOVER-1, "Ghz")
	require.True(t, feq == "9000M")
	feq = CpuFrequencyToString((FREQUENCYCUTOVER-1)*1000, "Mhz")
	require.True(t, feq == "9000M")
}
