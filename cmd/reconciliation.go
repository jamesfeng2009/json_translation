package cmd

import (
	"context"
	"json_trans_api/pkg/logger"
	"json_trans_api/pkg/reconciliation"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var reconciliationCmd = &cobra.Command{
	Use:   "reconciliation",
	Short: "Stripe reconciliation service",
	Long:  `Stripe reconciliation service for data consistency checking.`,
	Run: func(cmd *cobra.Command, args []string) {
		runReconciliationService()
	},
}

func init() {
	rootCmd.AddCommand(reconciliationCmd)
}

// runReconciliationService 运行对账服务
func runReconciliationService() {
	logger.Logger.Info("启动Stripe对账服务")

	// 创建对账服务
	service, err := reconciliation.NewReconciliationService()
	if err != nil {
		logger.Logger.Fatal("创建对账服务失败", "error", err.Error())
	}

	// 创建调度器
	scheduler := reconciliation.NewScheduler(service)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		logger.Logger.Fatal("启动调度器失败", "error", err.Error())
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 优雅关闭
	<-sigChan
	logger.Logger.Info("收到关闭信号，正在优雅关闭...")

	// 停止调度器
	scheduler.Stop()

	// 等待一段时间确保所有任务完成
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		logger.Logger.Warn("关闭超时，强制退出")
	default:
		logger.Logger.Info("对账服务已优雅关闭")
	}
} 