package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"alex/internal/config"
	"alex/internal/webui"
)

var webuiCmd = &cobra.Command{
	Use:   "webui",
	Short: "Start ALEX Web UI server",
	Long: `Start the ALEX Web UI server to provide HTTP API and WebSocket interface.
This allows you to interact with ALEX through a web browser or HTTP clients.`,
	Run: runWebUI,
}

var (
	webuiHost       string
	webuiPort       int
	webuiEnableCORS bool
	webuiDebug      bool
)

func init() {
	// 添加标志
	webuiCmd.Flags().StringVar(&webuiHost, "host", "localhost", "Host to bind the web server")
	webuiCmd.Flags().IntVar(&webuiPort, "port", 8080, "Port to bind the web server")
	webuiCmd.Flags().BoolVar(&webuiEnableCORS, "cors", true, "Enable CORS support")
	webuiCmd.Flags().BoolVar(&webuiDebug, "debug", false, "Enable debug mode")
}

func runWebUI(cmd *cobra.Command, args []string) {
	// 创建配置管理器
	configManager, err := config.NewManager()
	if err != nil {
		log.Fatalf("Failed to create config manager: %v", err)
	}

	// 创建服务器配置
	serverConfig := &webui.ServerConfig{
		Host:       webuiHost,
		Port:       webuiPort,
		EnableCORS: webuiEnableCORS,
		Debug:      webuiDebug,
	}

	// 创建Web UI服务器
	server, err := webui.NewServer(configManager, serverConfig)
	if err != nil {
		log.Fatalf("Failed to create web server: %v", err)
	}

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		fmt.Printf("🚀 Starting ALEX Web UI server on %s:%d\n", webuiHost, webuiPort)
		fmt.Printf("📖 API documentation: http://%s:%d/api/health\n", webuiHost, webuiPort)
		fmt.Printf("🔗 WebSocket endpoint: ws://%s:%d/api/sessions/{id}/stream\n", webuiHost, webuiPort)
		fmt.Printf("⏹️  Press Ctrl+C to stop\n\n")

		if err := server.Start(); err != nil {
			log.Printf("Web server error: %v", err)
			cancel()
		}
	}()

	// 等待关闭信号
	select {
	case <-sigChan:
		fmt.Println("\n🛑 Received shutdown signal, stopping server...")
	case <-ctx.Done():
		fmt.Println("\n🛑 Server context cancelled, stopping...")
	}

	// 优雅关闭
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	fmt.Println("✅ ALEX Web UI server stopped successfully")
}
