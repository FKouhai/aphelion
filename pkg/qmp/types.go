package qmp

const (
	cmdQueryStatus     = "query-status"
	cmdQueryCPUs       = "query-cpus-fast"
	cmdQueryMemory     = "query-memory-size-summary"
	cmdQueryBlockStats = "query-blockstats"
	cmdQueryChardev    = "query-chardev"
	cmdReset           = "system_reset"
	cmdPowerdown       = "system_powerdown"
	cmdStop            = "stop"
	cmdResume          = "cont"
)

// RunState represents the possible VM run states from QMP
type RunState string

const (
	RunStateRunning   RunState = "running"
	RunStatePaused    RunState = "paused"
	RunStateShutdown  RunState = "shutdown"
	RunStateSuspended RunState = "suspended"
	RunStatePrelaunch RunState = "prelaunch"
	RunStateInMigrate RunState = "inmigrate"
)

// VMStatus is the response from query-status
type VMStatus struct {
	Running    bool     `json:"running"`
	SingleStep bool     `json:"singlestep"`
	Status     RunState `json:"status"`
}

// CPU is an entry from query-cpus-fast
type CPU struct {
	Index    int    `json:"cpu-index"`
	ThreadID int    `json:"thread-id"`
	Target   string `json:"target"`
	QOMPath  string `json:"qom-path"`
}

// MemorySummary is the response from query-memory-size-summary
type MemorySummary struct {
	BaseMemory    uint64 `json:"base-memory"`
	PluggedMemory uint64 `json:"plugged-memory"`
}

// IOStats holds read/write counters from query-blockstats
type IOStats struct {
	ReadBytes  uint64 `json:"rd_bytes"`
	WriteBytes uint64 `json:"wr_bytes"`
	ReadOps    uint64 `json:"rd_operations"`
	WriteOps   uint64 `json:"wr_operations"`
}

// BlockStat is an entry from query-blockstats
type BlockStat struct {
	Device   string  `json:"device"`
	NodeName string  `json:"node-name"`
	Stats    IOStats `json:"stats"`
}

// CharDev is an entry from query-chardev
type CharDev struct {
	Label        string `json:"label"`
	Filename     string `json:"filename"`
	FrontendOpen bool   `json:"frontend-open"`
}
