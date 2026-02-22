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

// pythonRunnerScript is written to runner.py in every workdir.
// For each test case group in input.txt, it prepends the variable assignments
// to main.py and runs the combined file — so the user's code can reference
// nums, k, etc. directly without calling input().
// If input.txt is empty, main.py is run as-is.
const pythonRunnerScript = `#!/usr/bin/env python3
import subprocess, sys

with open('main.py') as f:
    solution = f.read()

with open('input.txt') as f:
    content = f.read().strip()

# Each test case is a block of "key=value" lines, groups separated by blank lines.
groups = [g.strip() for g in content.split('\n\n') if g.strip()]

if not groups:
    proc = subprocess.run(['python3', 'main.py'], text=True, capture_output=True)
    sys.stdout.write(proc.stdout)
    sys.stderr.write(proc.stderr)
    sys.exit(proc.returncode)

for group in groups:
    # Prepend variable assignments so the solution can use them directly.
    combined = group + '\n' + solution
    with open('run_case.py', 'w') as f:
        f.write(combined)
    try:
        proc = subprocess.run(
            ['python3', 'run_case.py'],
            text=True,
            capture_output=True,
            timeout=10,
        )
        sys.stdout.write(proc.stdout)
        sys.stderr.write(proc.stderr)
    except subprocess.TimeoutExpired:
        print('Time Limit Exceeded', file=sys.stderr)
        sys.exit(124)
`

// writeSourceFile writes main.py, the preprocessed input.txt, and runner.py.
func (executor *Python3JobExecutor) writeSourceFile(dir string, source model.SourceCode) error {
	if err := os.WriteFile(fmt.Sprintf("%s/main.py", dir), []byte(source.Content), fs.FileMode(0644)); err != nil {
		return err
	}
	if err := os.WriteFile(fmt.Sprintf("%s/input.txt", dir), []byte(preprocessTestCases(source.Input)), fs.FileMode(0644)); err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/runner.py", dir), []byte(pythonRunnerScript), fs.FileMode(0755))
}

// preprocessTestCases converts the UI input format into Python variable assignment blocks.
//
// UI format (one test case per line, variables separated by top-level commas):
//   nums=[1,2,4,5], k=3
//   nums=[1,2,4,9], k=6
//
// Produces (blank line between test cases, one assignment per line):
//   nums=[1,2,4,5]
//   k=3
//
//   nums=[1,2,4,9]
//   k=6
//
// Commas inside brackets are ignored — splitTopLevel handles nested structures.
func preprocessTestCases(raw string) string {
	var groups []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tokens := splitTopLevel(line)
		stmts := make([]string, 0, len(tokens))
		for _, tok := range tokens {
			stmts = append(stmts, strings.TrimSpace(tok))
		}
		groups = append(groups, strings.Join(stmts, "\n"))
	}
	return strings.Join(groups, "\n\n")
}

// splitTopLevel splits s by comma, ignoring commas nested inside [], {}, or ().
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	var cur strings.Builder
	for _, ch := range s {
		switch ch {
		case '(', '[', '{':
			depth++
			cur.WriteRune(ch)
		case ')', ']', '}':
			depth--
			cur.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(cur.String()))
				cur.Reset()
			} else {
				cur.WriteRune(ch)
			}
		default:
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, strings.TrimSpace(cur.String()))
	}
	return parts
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
		Cmd: []string{"sh", "-c", "timeout --foreground 30s python3 runner.py 2>&1 | head -c 8192"},
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
