package java_job_executor

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

// JavaJobExecutor handles compilation and execution of Java source code.
type JavaJobExecutor struct {
	cli *client.Client
	job_executor.JobExecutor
}

var instance *JavaJobExecutor
var once sync.Once

const (
	timeoutStatusCode      = 124
	compileErrorStatusCode = 100 // custom exit code emitted by the wrapper script below
)

var resourcesConf = container.Resources{
	Memory:   1073741824, // 1 GB of RAM
	CPUQuota: 100000,     // 1 CPU core
}

func (executor *JavaJobExecutor) Execute(source model.SourceCode) job_executor.JobExecutorOutput {
	dir, err := os.MkdirTemp("", "java-workdir")
	if err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to create temp dir: %v", err)}
	}
	defer os.RemoveAll(dir)

	if err := executor.writeSourceFile(dir, source); err != nil {
		return job_executor.JobExecutorOutput{Status: job_executor.RuntimeError, Output: fmt.Sprintf("Failed to write source file: %v", err)}
	}

	return executor.runExecutable(dir)
}

// writeSourceFile writes Main.java and input.txt to the working directory.
func (executor *JavaJobExecutor) writeSourceFile(dir string, source model.SourceCode) error {
	if err := os.WriteFile(fmt.Sprintf("%s/Main.java", dir), []byte(source.Content), fs.FileMode(0644)); err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/input.txt", dir), []byte(source.Input), fs.FileMode(0644))
}

// runExecutable compiles Main.java then runs it inside a Docker container.
// Exit code 100 is used as a sentinel for compile errors so they can be
// distinguished from runtime errors in the status mapping below.
func (executor *JavaJobExecutor) runExecutable(dir string) job_executor.JobExecutorOutput {
	ctx := context.Background()

	resp, err := executor.cli.ContainerCreate(ctx, &container.Config{
		Image:      "openjdk:17-slim",
		WorkingDir: "/workdir",
		Cmd: []string{
			"sh", "-c",
			"javac Main.java 2>&1; [ $? -ne 0 ] && exit 100; timeout --foreground 30s java Main < input.txt 2>&1 | head -c 8192",
		},
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
		case compileErrorStatusCode:
			status = job_executor.CompileError
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
		stdcopy.StdCopy(stdoutBuffer, stderrBuffer, attachResp.Reader)

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

func GetInstance() *JavaJobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			log.Fatal("Failed to create Docker client:", err)
		}
		instance = &JavaJobExecutor{cli: cli}
		instance.pullImage()
	})
	return instance
}

// pullImage pre-pulls the OpenJDK image so first executions aren't slow.
func (executor *JavaJobExecutor) pullImage() {
	ctx := context.Background()
	log.Println("Pulling image openjdk:17-slim ...")
	out, err := executor.cli.ImagePull(ctx, "docker.io/library/openjdk:17-slim", image.PullOptions{})
	if err != nil {
		log.Fatal("Failed to pull Java image:", err)
	}
	out.Close()
	log.Println("Java image ready.")
}
