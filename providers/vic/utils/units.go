package utils

import (
	"fmt"
	"strings"
)

const (
	BYTE = 1.0 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
	PETABYTE
)

const (
	KILOBYTESUFFIX = "Ki"
	MEGABYTESUFFIX = "Mi"
	GIGABYTESUFFIX = "Gi"
	TERABYTESUFFIX = "Ti"
	PETABYTESUFFIX = "Pi"
)

const (
	ONE  = 1.0
	KILO = 1000.0
	MEGA = KILO * KILO
	GIGA = MEGA * KILO
	TERA = GIGA * KILO
	PETA = TERA * KILO
)

const (
	KILOSUFFIX = "K"
	MEGASUFFIX = "M"
	GIGASUFFIX = "G"
	TERASUFFIX = "T"
	PETASUFFIX = "P"
)

const (
	MEMORYCUTOVER    = 100
	FREQUENCYCUTOVER = 10
)

const (
	MINPODSIZE = 2 * GIGABYTE
)

func MemsizeToBytesize(size int64, unit string) int64 {
	u := strings.ToLower(unit)
	var sizeInBytes int64
	switch u {
	case "b":
		sizeInBytes = size
		break
	case "k":
	case "kb":
		sizeInBytes = size * KILOBYTE
		break
	case "m":
	case "mb":
		sizeInBytes = size * MEGABYTE
		break
	case "g":
	case "gb":
		sizeInBytes = size * GIGABYTE
		break
	case "t":
	case "tb":
		sizeInBytes = size * TERABYTE
		break
	case "p":
	case "pb":
		sizeInBytes = size * PETABYTE
		break
	default:
		return 0
	}

	return sizeInBytes
}

func MemsizeToDecimalString(size int64, unit string) string {
	sizeInBytes := MemsizeToBytesize(size, unit)

	var res string
	if sizeInBytes >= PETA*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/PETA, PETASUFFIX)
	} else if sizeInBytes >= TERA*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/TERA, TERASUFFIX)
	} else if sizeInBytes >= GIGA*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/GIGA, GIGASUFFIX)
	} else if sizeInBytes >= MEGA*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/MEGA, MEGASUFFIX)
	} else if sizeInBytes >= KILO*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/KILO, KILOSUFFIX)
	} else {
		res = fmt.Sprintf("%d", sizeInBytes)
	}

	return res
}

func MemsizeToBinaryString(size int64, unit string) string {
	sizeInBytes := MemsizeToBytesize(size, unit)

	var res string
	if sizeInBytes >= PETABYTE*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/PETABYTE, PETABYTESUFFIX)
	} else if sizeInBytes >= TERABYTE*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/TERABYTE, TERABYTESUFFIX)
	} else if sizeInBytes >= GIGABYTE*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/GIGABYTE, GIGABYTESUFFIX)
	} else if sizeInBytes >= MEGABYTE*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/MEGABYTE, MEGABYTESUFFIX)
	} else if sizeInBytes >= KILOBYTE*MEMORYCUTOVER {
		res = fmt.Sprintf("%d%s", sizeInBytes/KILOBYTE, KILOBYTESUFFIX)
	} else {
		res = fmt.Sprintf("%d", sizeInBytes)
	}

	return res
}

func MemsizeToMaxPodCount(size int64, unit string) int64 {
	sizeInBytes := MemsizeToBytesize(size, unit)

	// Divide by minimum pod size
	return sizeInBytes / MINPODSIZE
}

func FrequencyToHertzFrequency(size int64, unit string) int64 {
	u := strings.ToLower(unit)
	var sizeInBytes int64
	switch u {
	case "k":
	case "khz":
		sizeInBytes = size * KILO
		break
	case "m":
	case "mhz":
		sizeInBytes = size * MEGA
		break
	case "g":
	case "ghz":
		sizeInBytes = size * GIGA
		break
	default:
		return 0
	}

	return sizeInBytes
}

func CpuFrequencyToString(size int64, unit string) string {
	var res string

	hertz := FrequencyToHertzFrequency(size, unit)
	if hertz >= GIGA*10 {
		res = fmt.Sprintf("%d%s", hertz/GIGA, GIGASUFFIX)
	} else if hertz >= MEGA*FREQUENCYCUTOVER {
		res = fmt.Sprintf("%d%s", hertz/MEGA, MEGASUFFIX)
	} else if hertz >= KILO*FREQUENCYCUTOVER {
		res = fmt.Sprintf("%d%s", hertz/KILO, KILOSUFFIX)
	} else {
		res = fmt.Sprintf("%d", hertz)
	}

	return res
}

func CpuFrequencyToCores(size int64, unit string) int64 {
	hertz := FrequencyToHertzFrequency(size, unit)
	// Assume 1G per Core
	cores := hertz / GIGA
	return cores
}
