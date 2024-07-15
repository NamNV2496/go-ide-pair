package job_executor

type ExecutionStatus int

const (
	NotExecuted ExecutionStatus = iota
	CompileError
	CompileTimeout
	RuntimeError
	RuntimeTimeout
	Successful
)
