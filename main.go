package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"miniarena/server"
)

// MiniArena 入口：启动 HTTP + WebSocket 服务，并初始化房间管理器
func main() {
	var addr string
	flag.StringVar(&addr, "addr", ":8080", "server listen address, e.g. :8080")
	flag.Parse()
	// 使用第三方 zap 日志库写入 app.log（带滚动）
	if err := server.InitLogger("app.log"); err != nil {
		panic(err)
	}
	defer server.SyncLogger()

	rm := server.GetRoomManager()
	// 先预创建一个默认房间，便于快速试跑
	_ = rm.GetOrCreateRoom("room-1")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWS)
	// 前后端分离：将 / 映射到 web 目录的静态资源
	mux.Handle("/", http.FileServer(http.Dir("web")))
	// 管理与监控接口
	mux.HandleFunc("/admin/config", server.HandleAdminConfig)
	mux.HandleFunc("/metrics", server.HandleMetrics)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		server.Log.Infof("MiniArena listening on %s; open http://localhost%v/", addr, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.Log.Fatalf("listen: %v", err)
		}
	}()

	// 优雅退出（Ctrl+C）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	server.Log.Info("Shutting down...")
}
