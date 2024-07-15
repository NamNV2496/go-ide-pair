package python3_job_executor

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"

	"github.com/araddon/dateparse"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/namnv2496/go-ide-pair/internal/executor/worker/job_executor"
	"github.com/namnv2496/go-ide-pair/internal/model"
)

// Python3JobExecutor handles code execution for Python source codes.
type Python3JobExecutor struct {
	cli client.Client
	job_executor.JobExecutor
}

var instance *Python3JobExecutor
var once sync.Once

func (executor *Python3JobExecutor) Execute(source model.SourceCode) job_executor.JobExecutorOutput {
	dir, err := os.MkdirTemp("", "workdir")
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to create temp dir: %v", err)}
	}
	defer os.RemoveAll(dir)

	executor.writeSourceFile(dir, source)

	return executor.runExecutable(dir, source)
}

// Write the source file to a temporary directory.
func (executor *Python3JobExecutor) writeSourceFile(dir string, source model.SourceCode) {
	sourceFilePath := fmt.Sprintf("%s/main.py", dir)

	sourceCode := source.Input + "\n" + source.Content
	err := os.WriteFile(sourceFilePath, []byte(sourceCode), fs.FileMode(0644))
	if err != nil {
		panic(err)
	}
}

var resourcesConf = container.Resources{
	Memory:   1073741824, // 1 GB of RAM
	CPUQuota: 100000,     // 1 CPU core
}

const timeoutStatusCode = 124

// Run the Python script.
func (executor *Python3JobExecutor) runExecutable(dir string, source model.SourceCode) job_executor.JobExecutorOutput {
	ctx := context.Background()

	resp, err := executor.cli.ContainerCreate(ctx, &container.Config{
		Image:        "python:3.9.19-slim-bullseye",
		WorkingDir:   "/workdir",
		Cmd:          []string{"timeout", "--foreground", "30s", "python3", "main.py", "|", "head", "-c", "8k"},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
	}, &container.HostConfig{
		Binds:     []string{fmt.Sprintf("%s:/workdir", dir)},
		Resources: resourcesConf,
	}, nil, nil, "")
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to create container: %v", err)}
	}

	defer func() {
		if err := executor.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); err != nil {
			panic(err)
		}
	}()

	attachResp, err := executor.cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to attach to container: %v", err)}
	}
	defer attachResp.Close()

	if err := executor.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to start container: %v", err)}
	}

	okChan, errChan := executor.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case data := <-okChan:
		inspectResp, err := executor.cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to inspect container: %v", err)}
		}

		var status job_executor.ExecutionStatus
		switch data.StatusCode {
		case 0:
			status = job_executor.Successful
		case timeoutStatusCode:
			status = job_executor.RuntimeTimeout
		default:
			status = job_executor.RuntimeError
		}

		startTime, err := dateparse.ParseAny(inspectResp.State.StartedAt)
		if err != nil {
			return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to parse start time: %v", err)}
		}
		finishTime, err := dateparse.ParseAny(inspectResp.State.FinishedAt)
		if err != nil {
			return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Failed to parse finish time: %v", err)}
		}
		runTime := finishTime.Sub(startTime).Milliseconds()

		stdoutBuffer := new(bytes.Buffer)
		stderrBuffer := new(bytes.Buffer)
		stdcopy.StdCopy(stdoutBuffer, stderrBuffer, attachResp.Reader)
		stdout := stdoutBuffer.String()

		return job_executor.JobExecutorOutput{
			Status: status,
			// ExitCode: exitCode,
			RunTime: runTime,
			Output:  stdout,
		}

	case err := <-errChan:
		return job_executor.JobExecutorOutput{Status: job_executor.ExecutionStatus(model.RuntimeError), Output: fmt.Sprintf("Container wait error: %v", err)}
	}
}

func GetInstance() *Python3JobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = &Python3JobExecutor{
			cli: *cli,
		}
		instance.pullImage()
	})
	return instance
}

// Prepare the necessary Docker images, to save time when handling jobs.
func (executor *Python3JobExecutor) pullImage() {
	ctx := context.Background()
	log.Println("Pull image python:3.9.19-slim-bullseye")
	_, err := executor.cli.ImagePull(ctx, "docker.io/library/python:3.9.19-slim-bullseye", image.PullOptions{})
	if err != nil {
		panic(err)
	}
}
