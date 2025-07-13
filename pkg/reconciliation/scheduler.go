package reconciliation

import (
	"fmt"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/logger"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 对账任务调度器
type Scheduler struct {
	cron     *cron.Cron
	service  *ReconciliationService
	entryID  cron.EntryID
}

// NewScheduler 创建调度器实例
func NewScheduler(service *ReconciliationService) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		service: service,
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	// 添加定时任务
	entryID, err := s.cron.AddFunc(s.service.config.Schedule, s.runReconciliationTask)
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %v", err)
	}
	s.entryID = entryID

	// 启动调度器
	s.cron.Start()
	logger.Logger.Info("对账调度器已启动", "schedule", s.service.config.Schedule)

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
		logger.Logger.Info("对账调度器已停止")
	}
}

// runReconciliationTask 执行对账任务
func (s *Scheduler) runReconciliationTask() {
	logger.Logger.Info("开始执行定时对账任务")

	// 计算对账时间范围（默认对账前一天的数据）
	endDate := time.Now().Truncate(24 * time.Hour)
	startDate := endDate.Add(-24 * time.Hour)

	// 执行对账
	report, err := s.service.RunReconciliation(startDate, endDate)
	if err != nil {
		logger.Logger.Error("定时对账任务执行失败", "error", err.Error())
		return
	}

	logger.Logger.Info("定时对账任务执行完成", 
		"report_id", report.ID,
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"))
}

// RunManualReconciliation 手动执行对账任务
func (s *Scheduler) RunManualReconciliation(startDate, endDate time.Time) (*tables.ReconciliationReport, error) {
	logger.Logger.Info("开始执行手动对账任务", 
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"))

	report, err := s.service.RunReconciliation(startDate, endDate)
	if err != nil {
		logger.Logger.Error("手动对账任务执行失败", "error", err.Error())
		return nil, err
	}

	logger.Logger.Info("手动对账任务执行完成", "report_id", report.ID)
	return report, nil
}

// GetNextRunTime 获取下次运行时间
func (s *Scheduler) GetNextRunTime() time.Time {
	if s.cron == nil {
		return time.Time{}
	}
	
	entries := s.cron.Entries()
	if len(entries) == 0 {
		return time.Time{}
	}
	
	return entries[0].Next
}

// GetSchedule 获取当前调度配置
func (s *Scheduler) GetSchedule() string {
	return s.service.config.Schedule
}

// UpdateSchedule 更新调度配置
func (s *Scheduler) UpdateSchedule(newSchedule string) error {
	// 停止当前任务
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}

	// 添加新任务
	entryID, err := s.cron.AddFunc(newSchedule, s.runReconciliationTask)
	if err != nil {
		return fmt.Errorf("更新调度配置失败: %v", err)
	}
	s.entryID = entryID

	// 更新配置
	s.service.config.Schedule = newSchedule
	s.service.config.UpdatedAt = time.Now()

	logger.Logger.Info("对账调度配置已更新", "new_schedule", newSchedule)
	return nil
} 