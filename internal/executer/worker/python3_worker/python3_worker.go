package python3_worker

import (
	"context"
	"os"
	"sync"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

var (
	once     sync.Once
	instance *Python3JobExecutor
)

type Python3JobExecutor struct {
	cli *client.Client
}

func NewPython3Instance() *Python3JobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = &Python3JobExecutor{
			cli: cli,
		}
		instance.prepareImage()
	})
	return instance
}

// Prepare the necessary Docker images, to save time when handling jobs.
func (executor *Python3JobExecutor) prepareImage() {
	env := os.Getenv("ENV_RUN")
	if env == "local" {
		cli := executor.cli
		ctx := context.Background()
		_, err := cli.ImagePull(ctx, "docker.io/library/python:3.9-buster", image.PullOptions{})
		if err != nil {
			panic(err)
		}
	}
}
