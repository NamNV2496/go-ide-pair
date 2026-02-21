package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	java_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/java_worker"
	python3_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/python3_worker"
	"github.com/namnv2496/go-ide-pair/internal/model"
)

func submitHandler(ctx *gin.Context) {
	var req model.SourceCode
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if len(req.Content) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if len(req.Content) > 8192 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "content exceeds 8192 character limit"})
		return
	}
	if len(req.Input) > 8192 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "input exceeds 8192 character limit"})
		return
	}

	switch req.Language {
	case model.Python3:
		ctx.JSON(http.StatusOK, python3_job_executor.GetInstance().Execute(req))
	case model.Java:
		ctx.JSON(http.StatusOK, java_job_executor.GetInstance().Execute(req))
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported language: %d", req.Language)})
	}
}
