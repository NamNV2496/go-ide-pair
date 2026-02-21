package python3_job_executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/araddon/dateparse"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/namnv2496/go-ide-pair/internal/executor/worker/job_executor"
	"github.com/namnv2496/go-ide-pair/internal/model"
)

const (
	PythonImage = "python:3.9.19-slim-bullseye"
)

// Python3JobExecutor handles code execution for Python source codes.
type Python3JobExecutor struct {
	cli *client.Client
	job_executor.JobExecutor
}

var instance *Python3JobExecutor
var once sync.Once

func (executor *Python3JobExecutor) Execute(source model.SourceCode) job_executor.JobExecutorOutput {
	dir, err := os.MkdirTemp("", "py-workdir")
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to create temp dir: %v", err)}
	}
	defer os.RemoveAll(dir)

	if err := executor.writeSourceFile(dir, source); err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to write source file: %v", err)}
	}

	return executor.runExecutable(dir)
}

// writeSourceFile writes the source code and formatted stdin to separate files.
func (executor *Python3JobExecutor) writeSourceFile(dir string, source model.SourceCode) error {
	// content := bytes.Join([][]byte{[]byte(formatStdin(source.Input)), []byte(source.Content)}, []byte("\n"))
	if err := os.WriteFile(fmt.Sprintf("%s/main.py", dir), []byte(source.Content), fs.FileMode(0644)); err != nil {
		return err
	}
	return nil
}

func formatStdin(raw string) string {
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, "\n")
}

var resourcesConf = container.Resources{
	Memory:   1073741824, // 1 GB of RAM
	CPUQuota: 100000,     // 1 CPU core
}

const timeoutStatusCode = 124

// runExecutable spins up a Docker container and runs main.py with input.txt on stdin.
func (executor *Python3JobExecutor) runExecutable(dir string) job_executor.JobExecutorOutput {
	ctx := context.Background()

	resp, err := executor.cli.ContainerCreate(ctx, &container.Config{
		Image:      PythonImage,
		WorkingDir: "/workdir",
		Cmd:        []string{"sh", "-c", "timeout --foreground 30s python3 main.py | head -c 8192"},
	}, &container.HostConfig{
		Binds:     []string{fmt.Sprintf("%s:/workdir", dir)},
		Resources: resourcesConf,
	}, nil, nil, "")
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to create container: %v", err)}
	}

	defer func() {
		if err := executor.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); err != nil {
			log.Printf("Warning: failed to remove container %s: %v", resp.ID, err)
		}
	}()

	attachResp, err := executor.cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to attach to container: %v", err)}
	}
	defer attachResp.Close()

	if err := executor.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to start container: %v", err)}
	}

	okChan, errChan := executor.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case data := <-okChan:
		inspectResp, err := executor.cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to inspect container: %v", err)}
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
			return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to parse start time: %v", err)}
		}
		finishTime, err := dateparse.ParseAny(inspectResp.State.FinishedAt)
		if err != nil {
			return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to parse finish time: %v", err)}
		}
		runTime := finishTime.Sub(startTime).Milliseconds()

		stdoutBuffer := new(bytes.Buffer)
		stderrBuffer := new(bytes.Buffer)
		if _, err := stdcopy.StdCopy(stdoutBuffer, stderrBuffer, attachResp.Reader); err != nil {
			log.Printf("Warning: stdcopy error: %v", err)
		}

		return job_executor.JobExecutorOutput{
			Status:   status,
			ExitCode: int(data.StatusCode),
			RunTime:  runTime,
			Output:   stdoutBuffer.String(),
		}

	case err := <-errChan:
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Container wait error: %v", err)}
	}
}

func GetInstance() *Python3JobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			log.Fatal("Failed to create Docker client:", err)
		}
		instance = &Python3JobExecutor{cli: cli}
		instance.pullImage()
	})
	return instance
}

// pullImage pre-pulls the Python image so first executions aren't slow.
// The response body MUST be fully drained before closing — otherwise Docker
// cancels the download mid-flight and the image is never stored locally.
func (executor *Python3JobExecutor) pullImage() {
	ctx := context.Background()
	log.Printf("Pulling image %s (this may take a minute on first run) ...", PythonImage)
	out, err := executor.cli.ImagePull(ctx, PythonImage, image.PullOptions{})
	if err != nil {
		log.Fatal("Failed to pull Python image:", err)
	}
	defer out.Close()
	if _, err := io.Copy(io.Discard, out); err != nil {
		log.Printf("Warning: error reading image pull stream: %v", err)
	}
	log.Println("Python image ready.")
}
