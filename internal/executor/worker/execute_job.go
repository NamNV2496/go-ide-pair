package worker

// import (
// 	"context"
// 	"log"
// 	"sync"

// 	worker "github.com/contribsys/faktory_worker_go"
// 	java_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/java_worker"
// 	"github.com/namnv2496/go-ide-pair/internal/executor/worker/job_executor"
// 	python3_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/python3_worker"
// 	"github.com/namnv2496/go-ide-pair/internal/model"
// 	"github.com/tranHieuDev23/IdeTwo/models/daos/execution_dao"
// 	"github.com/tranHieuDev23/IdeTwo/models/daos/source_code_dao"
// 	"github.com/tranHieuDev23/IdeTwo/utils/configs"
// )

// type FaktoryWorker struct {
// 	manager worker.Manager
// }

// func (worker *FaktoryWorker) Run() {
// 	worker.manager.Run()
// }

// var sourceDao = source_code_dao.GetInstance()
// var executionDao = execution_dao.GetInstance()

// func executeJob(ctx context.Context, args ...interface{}) error {

// 	var executor job_executor.JobExecutor
// 	switch source.Language {
// 	case model.Java:
// 		executor = java_job_executor.GetInstance()
// 	case model.Python3:
// 		executor = python3_job_executor.GetInstance()
// 	default:
// 		panic("Unsupported language")
// 	}

// 	output := executor.Execute(*source)

// 	exec.Status = output.Status
// 	exec.RunTime = output.RunTime
// 	exec.Output = output.Output
// 	executionDao.UpdateExecution(*exec)

// 	log.Printf("Job %s finished\n", helper.Jid())
// 	return nil
// }

// var instance *FaktoryWorker
// var once sync.Once
// var conf = configs.GetInstance()

// func GetInstance() *FaktoryWorker {
// 	once.Do(func() {
// 		manager := worker.NewManager()
// 		manager.Register("Execute Job", executeJob)
// 		manager.Concurrency = conf.FaktoryWorkerConcurrency
// 		manager.ProcessStrictPriorityQueues("critical", "default", "bulk")
// 		instance = &FaktoryWorker{
// 			manager: *manager,
// 		}
// 	})
// 	return instance
// }
