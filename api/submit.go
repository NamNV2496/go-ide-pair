package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	python3_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/python3_worker"
	"github.com/namnv2496/go-ide-pair/internal/model"
)

func submitHandler(ctx *gin.Context) {

	var req model.SourceCode
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fmt.Println("invalid input")
		return
	}
	executor := python3_job_executor.GetInstance()

	output := executor.Execute(req)
	ctx.JSON(http.StatusOK, output)
}
