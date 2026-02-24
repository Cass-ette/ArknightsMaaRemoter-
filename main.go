package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"ArknightsMaaRemoter/handler"
	staticfiles "ArknightsMaaRemoter/static"
	"ArknightsMaaRemoter/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	s := store.New()
	h := handler.New(s)

	r := gin.Default()

	// MAA 协议端点（匿名可访问，符合协议要求）
	r.POST("/maa/getTask", h.GetTask)
	r.POST("/maa/reportStatus", h.ReportStatus)

	// 管理端点（可通过 ADMIN_TOKEN 环境变量保护）
	admin := r.Group("/admin", h.AdminAuth())
	{
		admin.POST("/task", h.SubmitTask)
		admin.GET("/tasks", h.ListTasks)
		admin.GET("/screenshot/:id", h.GetScreenshot)
	}

	// 静态文件（内嵌于二进制，无需外部 static/ 目录）
	sub, _ := fs.Sub(staticfiles.FS, ".")
	r.StaticFS("/static", http.FS(sub))

	// 控制面板
	r.GET("/", h.Dashboard)

	log.Printf("MAA Remote 已启动，访问 http://localhost:%s", port)
	log.Printf("MAA 获取任务端点: http://localhost:%s/maa/getTask", port)
	log.Printf("MAA 汇报任务端点: http://localhost:%s/maa/reportStatus", port)
	log.Fatal(r.Run(":" + port))
}
