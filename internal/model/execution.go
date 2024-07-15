package model

type ExecutionStatus int

const (
	NotExecuted ExecutionStatus = iota
	CompileError
	CompileTimeout
	RuntimeError
	RuntimeTimeout
	Successful
)

type Execution struct {
	Timestamp int64           `json:"timestamp"`
	Status    ExecutionStatus `json:"status"`
	ExitCode  int             `json:"exitCode"`
	RunTime   int64           `json:"runTime"`
	Output    string          `json:"output"`
}
