package perfstat

type PartitionType struct {
	SmtCapable        bool /* OS supports SMT mode */
	SmtEnabled        bool /* SMT mode is on */
	LparCapable       bool /* OS supports logical partitioning */
	LparEnabled       bool /* logical partitioning is on */
	SharedCapable     bool /* OS supports shared processor LPAR */
	SharedEnabled     bool /* partition runs in shared mode */
	DLparCapable      bool /* OS supports dynamic LPAR */
	Capped            bool /* partition is capped */
	Kernel64bit       bool /* kernel is 64 bit */
	PoolUtilAuthority bool /* pool utilization available */
	DonateCapable     bool /* capable of donating cycles */
	DonateEnabled     bool /* enabled for donating cycles */
	AmsCapable        bool /* 1 = AMS(Active Memory Sharing) capable, 0 = Not AMS capable */
	AmsEnabled        bool /* 1 = AMS(Active Memory Sharing) enabled, 0 = Not AMS enabled */
	PowerSave         bool /*1= Power saving mode is enabled*/
	AmeEnabled        bool /* Active Memory Expansion is enabled */
	SharedExtended    bool
}

type PartitionValue struct {
	Online  int64
	Max     int64
	Min     int64
	Desired int64
}

type PartitionConfig struct {
	Version                  int64          /* Version number */
	Name                     string         /* Partition Name */
	Node                     string         /* Node Name */
	Conf                     PartitionType  /* Partition Properties */
	Number                   int32          /* Partition Number */
	GroupID                  int32          /* Group ID */
	ProcessorFamily          string         /* Processor Type */
	ProcessorModel           string         /* Processor Model */
	MachineID                string         /* Machine ID */
	ProcessorMhz             float64        /* Processor Clock Speed in MHz */
	NumProcessors            PartitionValue /* Number of Configured Physical Processors in frame*/
	OSName                   string         /* Name of Operating System */
	OSVersion                string         /* Version of operating System */
	OSBuild                  string         /* Build of Operating System */
	LCpus                    int32          /* Number of Logical CPUs */
	SmtThreads               int32          /* Number of SMT Threads */
	Drives                   int32          /* Total Number of Drives */
	NetworkAdapters          int32          /* Total Number of Network Adapters */
	CpuCap                   PartitionValue /* Min, Max and Online CPU Capacity */
	Weightage                int32          /* Variable Processor Capacity Weightage */
	EntCapacity              int32          /* number of processor units this partition is entitled to receive */
	VCpus                    PartitionValue /* Min, Max and Online Virtual CPUs */
	PoolID                   int32          /* Shared Pool ID of physical processors, to which this partition belongs*/
	ActiveCpusInPool         int32          /* Count of physical CPUs in the shared processor pool, to which this partition belongs */
	PoolWeightage            int32          /* Pool Weightage */
	SharedPCpu               int32          /* Number of physical processors allocated for shared processor use */
	MaxPoolCap               int32          /* Maximum processor capacity of partition's pool */
	EntPoolCap               int32          /* Entitled processor capacity of partition's pool */
	Mem                      PartitionValue /* Min, Max and Online Memory */
	MemWeightage             int32          /* Variable Memory Capacity Weightage */
	TotalIOMemoryEntitlement int64          /* I/O Memory Entitlement of the partition in bytes */
	MemPoolID                int32          /* AMS pool id of the pool the LPAR belongs to */
	HyperPgSize              int64          /* Hypervisor page size in KB*/
	ExpMem                   PartitionValue /* Min, Max and Online Expanded Memory */
	TargetMemExpFactor       int64          /* Target Memory Expansion Factor scaled by 100 */
	TargetMemExpSize         int64          /* Expanded Memory Size in MB */
	SubProcessorMode         int32          /* Split core mode, its value can be 0,1,2 or 4. 0 for unsupported, 1 for capable but not enabled, 2 or 4 for enabled*/
}
