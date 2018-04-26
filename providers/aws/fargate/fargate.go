package fargate

const (
	// EC2 compute resource units.

	// VCPU is one virtual CPU core in EC2.
	VCPU int64 = 1024
	// MiB is 2^20 bytes.
	MiB int64 = 1024 * 1024
	// GiB is 2^30 bytes.
	GiB int64 = 1024 * MiB
)

// TaskSize represents a Fargate task size.
type taskSize struct {
	cpu    int64
	memory memorySizeRange
}

// MemorySizeRange represents a range of Fargate task memory sizes.
type memorySizeRange struct {
	min int64
	max int64
	inc int64
}

var (
	// Fargate task size table.
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size
	//
	// VCPU     Memory (in MiBs, available in 1GiB increments)
	// ====     ===================
	//  256     512, 1024 ...  2048
	//  512          1024 ...  4096
	// 1024          2048 ...  8192
	// 2048          4096 ... 16384
	// 4096          8192 ... 30720
	//
	taskSizeTable = []taskSize{
		{VCPU / 4, memorySizeRange{512 * MiB, 512 * MiB, 1}},
		{VCPU / 4, memorySizeRange{1 * GiB, 2 * GiB, 1 * GiB}},
		{VCPU / 2, memorySizeRange{1 * GiB, 4 * GiB, 1 * GiB}},
		{1 * VCPU, memorySizeRange{2 * GiB, 8 * GiB, 1 * GiB}},
		{2 * VCPU, memorySizeRange{4 * GiB, 16 * GiB, 1 * GiB}},
		{4 * VCPU, memorySizeRange{8 * GiB, 30 * GiB, 1 * GiB}},
	}
)
