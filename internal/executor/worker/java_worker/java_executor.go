package java_job_executor

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/namnv2496/go-ide-pair/internal/executor/worker/job_executor"
	"github.com/namnv2496/go-ide-pair/internal/model"
)

// Logic to handle code execution for Java source codes.
type JavaJobExecutor struct {
	cli client.Client
	job_executor.JobExecutor
}

var instance *JavaJobExecutor
var once sync.Once

func (executor JavaJobExecutor) Execute(source model.SourceCode) job_executor.JobExecutorOutput {
	// dir := tempdir.New(conf.IdeTwoExecutionsDir)
	// defer dir.Close()

	// executor.writeSourceFile(dir, source)

	// if err := executor.compileSourceFile(dir, source); err != nil {
	// 	return *err
	// }

	// return *executor.runExecutable(dir, source)
	return job_executor.JobExecutorOutput{}
}

func GetInstance() *JavaJobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = &JavaJobExecutor{
			cli: *cli,
		}
		instance.pullImage()
	})
	return instance
}

// Prepare the necessary Docker images, to save time when handling jobs.
func (executor *JavaJobExecutor) pullImage() {
	ctx := context.Background()
	_, err := executor.cli.ImagePull(ctx, "docker.io/library/openjdk:13-buster", image.PullOptions{})
	if err != nil {
		panic(err)
	}
}
