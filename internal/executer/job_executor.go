package job_executor

import (
	"github.com/namnv2496/go-ide-pair/internal/model"
)

type JobExecutorOutput struct {
	Status   model.ExecutionStatus
	ExitCode int
	RunTime  int64
	Output   string
}

type JobExecutor interface {
	Execute(source model.SourceCode) JobExecutorOutput
}
