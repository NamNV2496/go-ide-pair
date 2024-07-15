package java_worker

import (
	"context"
	"os"
	"sync"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

var (
	once     sync.Once
	instance *JavaJobExecutor
)

type JavaJobExecutor struct {
	cli *client.Client
}

func NewJavaInstance() *JavaJobExecutor {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = &JavaJobExecutor{
			cli: cli,
		}
		instance.prepareImage()
	})
	return instance
}

// Prepare the necessary Docker images, to save time when handling jobs.
func (executor *JavaJobExecutor) prepareImage() {
	env := os.Getenv("ENV_RUN")
	if env == "local" {
		cli := executor.cli
		ctx := context.Background()
		_, err := cli.ImagePull(ctx, "docker.io/library/openjdk:13-buster", image.PullOptions{})
		if err != nil {
			panic(err)
		}
	}
}
