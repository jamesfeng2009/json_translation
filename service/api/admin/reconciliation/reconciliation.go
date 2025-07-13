package reconciliation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"json_trans_api/config"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/httpclient"
	"json_trans_api/pkg/logger"
	"json_trans_api/pkg/reconciliation"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler 对账API处理器
type Handler struct {
	service   *reconciliation.ReconciliationService
	scheduler *reconciliation.Scheduler
}

// NewHandler 创建处理器实例
func NewHandler() (*Handler, error) {
	service, err := reconciliation.NewReconciliationService()
	if err != nil {
		return nil, err
	}

	scheduler := reconciliation.NewScheduler(service)

	return &Handler{
		service:   service,
		scheduler: scheduler,
	}, nil
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/reconciliation", func(r chi.Router) {
		// 对账报告相关
		r.Get("/reports", h.GetReconciliationReports)
		r.Get("/reports/{reportID}", h.GetReconciliationReport)
		r.Get("/reports/{reportID}/diffs", h.GetReconciliationDiffs)
		r.Get("/reports/{reportID}/audit-logs", h.GetReconciliationAuditLogs)

		// 对账任务相关
		r.Post("/run", h.RunReconciliation)
		r.Post("/run-manual", h.RunManualReconciliation)
		r.Get("/status", h.GetReconciliationStatus)
		r.Post("/schedule", h.UpdateSchedule)

		// 差异管理
		r.Patch("/diffs/{diffID}/fix", h.FixDiff)
		r.Patch("/diffs/{diffID}/ignore", h.IgnoreDiff)
		r.Get("/diffs", h.GetAllDiffs)

		// 配置管理
		r.Get("/config", h.GetReconciliationConfig)
		r.Put("/config", h.UpdateReconciliationConfig)
	})
}

// GetReconciliationReports 获取对账报告列表
func (h *Handler) GetReconciliationReports(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	status := r.URL.Query().Get("status")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// 构建查询条件
	queryParams := fmt.Sprintf("?select=*&order=created_at.desc&limit=%d&offset=%d", limit, (page-1)*limit)
	
	if status != "" {
		queryParams += fmt.Sprintf("&status=eq.%s", status)
	}
	if startDate != "" {
		queryParams += fmt.Sprintf("&report_date=gte.%s", startDate)
	}
	if endDate != "" {
		queryParams += fmt.Sprintf("&report_date=lte.%s", endDate)
	}

	// 查询数据库
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_reports%s", config.Cfg.Supabase.SupabaseUrl, queryParams)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询对账报告失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var reports []tables.ReconciliationReport
	if err := json.NewDecoder(resp.Body).Decode(&reports); err != nil {
		http.Error(w, "解析对账报告失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"reports": reports,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetReconciliationReport 获取单个对账报告
func (h *Handler) GetReconciliationReport(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		http.Error(w, "报告ID不能为空", http.StatusBadRequest)
		return
	}

	// 查询数据库
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_reports?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, reportID)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询对账报告失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var reports []tables.ReconciliationReport
	if err := json.NewDecoder(resp.Body).Decode(&reports); err != nil {
		http.Error(w, "解析对账报告失败", http.StatusInternalServerError)
		return
	}

	if len(reports) == 0 {
		http.Error(w, "对账报告不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports[0])
}

// GetReconciliationDiffs 获取对账差异列表
func (h *Handler) GetReconciliationDiffs(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		http.Error(w, "报告ID不能为空", http.StatusBadRequest)
		return
	}

	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	recordType := r.URL.Query().Get("record_type")

	// 构建查询条件
	queryParams := fmt.Sprintf("?select=*&report_id=eq.%s&order=created_at.desc&limit=%d&offset=%d", reportID, limit, (page-1)*limit)
	
	if status != "" {
		queryParams += fmt.Sprintf("&status=eq.%s", status)
	}
	if severity != "" {
		queryParams += fmt.Sprintf("&severity=eq.%s", severity)
	}
	if recordType != "" {
		queryParams += fmt.Sprintf("&record_type=eq.%s", recordType)
	}

	// 查询数据库
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs%s", config.Cfg.Supabase.SupabaseUrl, queryParams)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询对账差异失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var diffs []tables.ReconciliationDiff
	if err := json.NewDecoder(resp.Body).Decode(&diffs); err != nil {
		http.Error(w, "解析对账差异失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"diffs": diffs,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetReconciliationAuditLogs 获取对账审计日志
func (h *Handler) GetReconciliationAuditLogs(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	if reportID == "" {
		http.Error(w, "报告ID不能为空", http.StatusBadRequest)
		return
	}

	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// 构建查询条件
	queryParams := fmt.Sprintf("?select=*&report_id=eq.%s&order=created_at.desc&limit=%d&offset=%d", reportID, limit, (page-1)*limit)

	// 查询数据库
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_audit_logs%s", config.Cfg.Supabase.SupabaseUrl, queryParams)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询审计日志失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var logs []tables.ReconciliationAuditLog
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		http.Error(w, "解析审计日志失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"audit_logs": logs,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RunReconciliation 执行对账任务
func (h *Handler) RunReconciliation(w http.ResponseWriter, r *http.Request) {
	// 计算对账时间范围（默认对账前一天的数据）
	endDate := time.Now().Truncate(24 * time.Hour)
	startDate := endDate.Add(-24 * time.Hour)

	// 执行对账
	report, err := h.service.RunReconciliation(startDate, endDate)
	if err != nil {
		logger.Logger.Error("执行对账任务失败", "error", err.Error())
		http.Error(w, "执行对账任务失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"message": "对账任务执行成功",
		"report":  report,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RunManualReconciliation 手动执行对账任务
func (h *Handler) RunManualReconciliation(w http.ResponseWriter, r *http.Request) {
	var request struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "解析请求参数失败", http.StatusBadRequest)
		return
	}

	// 解析日期
	startDate, err := time.Parse("2006-01-02", request.StartDate)
	if err != nil {
		http.Error(w, "开始日期格式错误", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", request.EndDate)
	if err != nil {
		http.Error(w, "结束日期格式错误", http.StatusBadRequest)
		return
	}

	// 执行对账
	report, err := h.scheduler.RunManualReconciliation(startDate, endDate)
	if err != nil {
		logger.Logger.Error("执行手动对账任务失败", "error", err.Error())
		http.Error(w, "执行对账任务失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"message": "手动对账任务执行成功",
		"report":  report,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetReconciliationStatus 获取对账状态
func (h *Handler) GetReconciliationStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"enabled":      h.service.config.Enabled,
		"schedule":     h.scheduler.GetSchedule(),
		"next_run":     h.scheduler.GetNextRunTime(),
		"auto_fix":     h.service.config.AutoFixEnabled,
		"notification": h.service.config.NotificationEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// UpdateSchedule 更新调度配置
func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Schedule string `json:"schedule"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "解析请求参数失败", http.StatusBadRequest)
		return
	}

	if request.Schedule == "" {
		http.Error(w, "调度配置不能为空", http.StatusBadRequest)
		return
	}

	// 更新调度配置
	err := h.scheduler.UpdateSchedule(request.Schedule)
	if err != nil {
		logger.Logger.Error("更新调度配置失败", "error", err.Error())
		http.Error(w, "更新调度配置失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"message":  "调度配置更新成功",
		"schedule": request.Schedule,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// FixDiff 修复差异
func (h *Handler) FixDiff(w http.ResponseWriter, r *http.Request) {
	diffID := chi.URLParam(r, "diffID")
	if diffID == "" {
		http.Error(w, "差异ID不能为空", http.StatusBadRequest)
		return
	}

	var request struct {
		Notes string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "解析请求参数失败", http.StatusBadRequest)
		return
	}

	// 获取差异记录
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, diffID)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询差异记录失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var diffs []tables.ReconciliationDiff
	if err := json.NewDecoder(resp.Body).Decode(&diffs); err != nil {
		http.Error(w, "解析差异记录失败", http.StatusInternalServerError)
		return
	}

	if len(diffs) == 0 {
		http.Error(w, "差异记录不存在", http.StatusNotFound)
		return
	}

	diff := diffs[0]

	// 修复差异
	err = h.service.fixDiff(&diff)
	if err != nil {
		logger.Logger.Error("修复差异失败", "diff_id", diffID, "error", err.Error())
		http.Error(w, "修复差异失败", http.StatusInternalServerError)
		return
	}

	// 更新差异状态
	diff.Status = "manual_fixed"
	diff.FixedAt = &time.Time{}
	*diff.FixedAt = time.Now()
	diff.FixedBy = "admin"
	diff.Notes = request.Notes
	diff.UpdatedAt = time.Now()

	err = h.service.updateReconciliationDiff(&diff)
	if err != nil {
		logger.Logger.Error("更新差异状态失败", "diff_id", diffID, "error", err.Error())
		http.Error(w, "更新差异状态失败", http.StatusInternalServerError)
		return
	}

	// 记录审计日志
	h.service.logAuditEvent(diff.ReportID, "manual_fix", diff.RecordType, diff.RecordID, diff.UserID, fmt.Sprintf("手动修复字段 %s: %s -> %s", diff.FieldName, diff.LocalValue, diff.StripeValue))

	// 返回响应
	response := map[string]interface{}{
		"message": "差异修复成功",
		"diff":    diff,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// IgnoreDiff 忽略差异
func (h *Handler) IgnoreDiff(w http.ResponseWriter, r *http.Request) {
	diffID := chi.URLParam(r, "diffID")
	if diffID == "" {
		http.Error(w, "差异ID不能为空", http.StatusBadRequest)
		return
	}

	var request struct {
		Notes string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "解析请求参数失败", http.StatusBadRequest)
		return
	}

	// 更新差异状态
	updateData := map[string]interface{}{
		"status":     "ignored",
		"notes":      request.Notes,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, diffID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "忽略差异失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 返回响应
	response := map[string]interface{}{
		"message": "差异已忽略",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetAllDiffs 获取所有差异
func (h *Handler) GetAllDiffs(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	recordType := r.URL.Query().Get("record_type")

	// 构建查询条件
	queryParams := fmt.Sprintf("?select=*&order=created_at.desc&limit=%d&offset=%d", limit, (page-1)*limit)
	
	if status != "" {
		queryParams += fmt.Sprintf("&status=eq.%s", status)
	}
	if severity != "" {
		queryParams += fmt.Sprintf("&severity=eq.%s", severity)
	}
	if recordType != "" {
		queryParams += fmt.Sprintf("&record_type=eq.%s", recordType)
	}

	// 查询数据库
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs%s", config.Cfg.Supabase.SupabaseUrl, queryParams)
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		http.Error(w, "查询差异失败", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var diffs []tables.ReconciliationDiff
	if err := json.NewDecoder(resp.Body).Decode(&diffs); err != nil {
		http.Error(w, "解析差异失败", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := map[string]interface{}{
		"diffs": diffs,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetReconciliationConfig 获取对账配置
func (h *Handler) GetReconciliationConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.service.config)
}

// UpdateReconciliationConfig 更新对账配置
func (h *Handler) UpdateReconciliationConfig(w http.ResponseWriter, r *http.Request) {
	var config tables.ReconciliationConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "解析配置失败", http.StatusBadRequest)
		return
	}

	// 更新配置
	*h.service.config = config
	h.service.config.UpdatedAt = time.Now()

	// 返回响应
	response := map[string]interface{}{
		"message": "配置更新成功",
		"config":  h.service.config,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
} 