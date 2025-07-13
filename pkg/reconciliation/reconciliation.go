package reconciliation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"json_trans_api/config"
	"json_trans_api/models/tables"
	"json_trans_api/pkg/httpclient"
	"json_trans_api/pkg/logger"
	stripeService "json_trans_api/pkg/stripe"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v72"
)

// ReconciliationService 对账服务
type ReconciliationService struct {
	config *tables.ReconciliationConfig
}

// NewReconciliationService 创建对账服务实例
func NewReconciliationService() (*ReconciliationService, error) {
	config, err := getReconciliationConfig()
	if err != nil {
		return nil, fmt.Errorf("获取对账配置失败: %v", err)
	}
	
	return &ReconciliationService{
		config: config,
	}, nil
}

// RunReconciliation 执行对账任务
func (rs *ReconciliationService) RunReconciliation(startDate, endDate time.Time) (*tables.ReconciliationReport, error) {
	// 创建对账报告
	report, err := rs.createReconciliationReport(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("创建对账报告失败: %v", err)
	}

	// 记录审计日志
	rs.logAuditEvent(report.ID, "reconciliation_started", "", "", "", fmt.Sprintf("开始对账，时间范围: %s 到 %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))

	// 更新报告状态为运行中
	rs.updateReportStatus(report.ID, "running")

	// 执行订阅对账
	subscriptionDiffs, err := rs.reconcileSubscriptions(startDate, endDate, report.ID)
	if err != nil {
		logger.Logger.Error("订阅对账失败", "error", err.Error())
	}

	// 执行发票对账
	invoiceDiffs, err := rs.reconcileInvoices(startDate, endDate, report.ID)
	if err != nil {
		logger.Logger.Error("发票对账失败", "error", err.Error())
	}

	// 执行客户对账
	customerDiffs, err := rs.reconcileCustomers(startDate, endDate, report.ID)
	if err != nil {
		logger.Logger.Error("客户对账失败", "error", err.Error())
	}

	// 统计差异
	totalDiffs := len(subscriptionDiffs) + len(invoiceDiffs) + len(customerDiffs)
	matchedRecords := rs.calculateMatchedRecords(startDate, endDate) - totalDiffs

	// 自动修复（如果启用）
	if rs.config.AutoFixEnabled {
		rs.autoFixDiffs(append(append(subscriptionDiffs, invoiceDiffs...), customerDiffs...))
	}

	// 更新报告状态
	rs.updateReportStats(report.ID, totalDiffs, matchedRecords, len(subscriptionDiffs), len(invoiceDiffs), len(customerDiffs))

	// 发送通知（如果启用）
	if rs.config.NotificationEnabled && totalDiffs > 0 {
		rs.sendReconciliationNotification(report.ID, totalDiffs)
	}

	// 记录审计日志
	rs.logAuditEvent(report.ID, "reconciliation_completed", "", "", "", fmt.Sprintf("对账完成，发现 %d 个差异", totalDiffs))

	return report, nil
}

// reconcileSubscriptions 对账订阅数据
func (rs *ReconciliationService) reconcileSubscriptions(startDate, endDate time.Time, reportID string) ([]*tables.ReconciliationDiff, error) {
	var diffs []*tables.ReconciliationDiff

	// 获取本地订阅数据
	localSubscriptions, err := rs.getLocalSubscriptions(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取本地订阅数据失败: %v", err)
	}

	// 对比每个订阅
	for _, localSub := range localSubscriptions {
		// 从Stripe获取订阅数据
		stripeSub, err := stripeService.GetSubscription(localSub.SubscriptionID)
		if err != nil {
			// 记录缺失的订阅
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "subscription",
				RecordID:   localSub.SubscriptionID,
				UserID:     localSub.UserID,
				DiffType:   "missing",
				FieldName:  "subscription",
				LocalValue: "exists",
				StripeValue: "not_found",
				Severity:   "high",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
			continue
		}

		// 对比订阅状态
		if string(stripeSub.Status) != localSub.Status {
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "subscription",
				RecordID:   localSub.SubscriptionID,
				UserID:     localSub.UserID,
				DiffType:   "mismatch",
				FieldName:  "status",
				LocalValue: localSub.Status,
				StripeValue: string(stripeSub.Status),
				Severity:   "medium",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
		}
	}

	return diffs, nil
}

// reconcileInvoices 对账发票数据
func (rs *ReconciliationService) reconcileInvoices(startDate, endDate time.Time, reportID string) ([]*tables.ReconciliationDiff, error) {
	var diffs []*tables.ReconciliationDiff

	// 获取本地发票数据
	localInvoices, err := rs.getLocalInvoices(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取本地发票数据失败: %v", err)
	}

	// 对比每个发票
	for _, localInvoice := range localInvoices {
		// 从Stripe获取发票数据
		stripeInvoice, err := stripeService.GetInvoice(localInvoice.InvoiceID)
		if err != nil {
			// 记录缺失的发票
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "invoice",
				RecordID:   localInvoice.InvoiceID,
				UserID:     localInvoice.UserID,
				DiffType:   "missing",
				FieldName:  "invoice",
				LocalValue: "exists",
				StripeValue: "not_found",
				Severity:   "medium",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
			continue
		}

		// 对比发票金额
		if int(stripeInvoice.AmountPaid) != localInvoice.Amount {
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "invoice",
				RecordID:   localInvoice.InvoiceID,
				UserID:     localInvoice.UserID,
				DiffType:   "mismatch",
				FieldName:  "amount",
				LocalValue: strconv.Itoa(localInvoice.Amount),
				StripeValue: strconv.FormatInt(stripeInvoice.AmountPaid, 10),
				Severity:   "high",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
		}
	}

	return diffs, nil
}

// reconcileCustomers 对账客户数据
func (rs *ReconciliationService) reconcileCustomers(startDate, endDate time.Time, reportID string) ([]*tables.ReconciliationDiff, error) {
	var diffs []*tables.ReconciliationDiff

	// 获取本地客户数据
	localCustomers, err := rs.getLocalCustomers(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取本地客户数据失败: %v", err)
	}

	// 对比每个客户
	for _, localCustomer := range localCustomers {
		// 从Stripe获取客户数据
		stripeCustomer, err := stripeService.GetCustomer(localCustomer.CustomerID)
		if err != nil {
			// 记录缺失的客户
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "customer",
				RecordID:   localCustomer.CustomerID,
				UserID:     localCustomer.UserID,
				DiffType:   "missing",
				FieldName:  "customer",
				LocalValue: "exists",
				StripeValue: "not_found",
				Severity:   "high",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
			continue
		}

		// 对比客户邮箱
		if stripeCustomer.Email != localCustomer.Email {
			diff := &tables.ReconciliationDiff{
				ID:         uuid.New().String(),
				ReportID:   reportID,
				RecordType: "customer",
				RecordID:   localCustomer.CustomerID,
				UserID:     localCustomer.UserID,
				DiffType:   "mismatch",
				FieldName:  "email",
				LocalValue: localCustomer.Email,
				StripeValue: stripeCustomer.Email,
				Severity:   "low",
				Status:     "pending",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			diffs = append(diffs, diff)
			rs.saveReconciliationDiff(diff)
		}
	}

	return diffs, nil
}

// autoFixDiffs 自动修复差异
func (rs *ReconciliationService) autoFixDiffs(diffs []*tables.ReconciliationDiff) {
	for _, diff := range diffs {
		// 检查是否满足自动修复条件
		if rs.shouldAutoFix(diff) {
			err := rs.fixDiff(diff)
			if err != nil {
				logger.Logger.Error("自动修复差异失败", "diff_id", diff.ID, "error", err.Error())
				continue
			}

			// 更新差异状态
			diff.Status = "auto_fixed"
			diff.AutoFixed = true
			diff.FixedAt = &time.Time{}
			*diff.FixedAt = time.Now()
			diff.FixedBy = "system"
			diff.UpdatedAt = time.Now()

			rs.updateReconciliationDiff(diff)

			// 记录审计日志
			rs.logAuditEvent(diff.ReportID, "auto_fix", diff.RecordType, diff.RecordID, diff.UserID, fmt.Sprintf("自动修复字段 %s: %s -> %s", diff.FieldName, diff.LocalValue, diff.StripeValue))
		}
	}
}

// shouldAutoFix 判断是否应该自动修复
func (rs *ReconciliationService) shouldAutoFix(diff *tables.ReconciliationDiff) bool {
	if !rs.config.AutoFixEnabled {
		return false
	}

	// 根据严重程度判断
	switch rs.config.AutoFixSeverity {
	case "low":
		return diff.Severity == "low"
	case "medium":
		return diff.Severity == "low" || diff.Severity == "medium"
	case "high":
		return diff.Severity == "low" || diff.Severity == "medium" || diff.Severity == "high"
	case "critical":
		return true
	default:
		return false
	}
}

// fixDiff 修复差异
func (rs *ReconciliationService) fixDiff(diff *tables.ReconciliationDiff) error {
	switch diff.RecordType {
	case "subscription":
		return rs.fixSubscriptionDiff(diff)
	case "invoice":
		return rs.fixInvoiceDiff(diff)
	case "customer":
		return rs.fixCustomerDiff(diff)
	default:
		return fmt.Errorf("不支持的记录类型: %s", diff.RecordType)
	}
}

// fixSubscriptionDiff 修复订阅差异
func (rs *ReconciliationService) fixSubscriptionDiff(diff *tables.ReconciliationDiff) error {
	switch diff.FieldName {
	case "status":
		// 更新本地订阅状态
		return rs.updateLocalSubscriptionStatus(diff.RecordID, diff.StripeValue)
	default:
		return fmt.Errorf("不支持的订阅字段: %s", diff.FieldName)
	}
}

// fixInvoiceDiff 修复发票差异
func (rs *ReconciliationService) fixInvoiceDiff(diff *tables.ReconciliationDiff) error {
	switch diff.FieldName {
	case "amount":
		// 更新本地发票金额
		amount, _ := strconv.Atoi(diff.StripeValue)
		return rs.updateLocalInvoiceAmount(diff.RecordID, amount)
	case "status":
		// 更新本地发票状态
		return rs.updateLocalInvoiceStatus(diff.RecordID, diff.StripeValue)
	default:
		return fmt.Errorf("不支持的发票字段: %s", diff.FieldName)
	}
}

// fixCustomerDiff 修复客户差异
func (rs *ReconciliationService) fixCustomerDiff(diff *tables.ReconciliationDiff) error {
	switch diff.FieldName {
	case "email":
		// 更新本地客户邮箱
		return rs.updateLocalCustomerEmail(diff.RecordID, diff.StripeValue)
	default:
		return fmt.Errorf("不支持的客户字段: %s", diff.FieldName)
	}
}

// 数据库操作辅助函数
func (rs *ReconciliationService) createReconciliationReport(startDate, endDate time.Time) (*tables.ReconciliationReport, error) {
	report := &tables.ReconciliationReport{
		ID:         uuid.New().String(),
		ReportDate: time.Now(),
		StartDate:  startDate,
		EndDate:    endDate,
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	reportData := map[string]interface{}{
		"id":          report.ID,
		"report_date": report.ReportDate,
		"start_date":  report.StartDate,
		"end_date":    report.EndDate,
		"status":      report.Status,
		"created_at":  report.CreatedAt,
		"updated_at":  report.UpdatedAt,
	}

	jsonData, _ := json.Marshal(reportData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_reports", config.Cfg.Supabase.SupabaseUrl)
	req, _ := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("创建对账报告失败: %d", resp.StatusCode)
	}

	return report, nil
}

func (rs *ReconciliationService) updateReportStatus(reportID, status string) error {
	updateData := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_reports?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, reportID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) updateReportStats(reportID string, totalDiffs, matchedRecords, subscriptionDiffs, invoiceDiffs, customerDiffs int) error {
	updateData := map[string]interface{}{
		"status":             "completed",
		"total_records":      totalDiffs + matchedRecords,
		"matched_records":    matchedRecords,
		"mismatched_records": totalDiffs,
		"missing_records":    subscriptionDiffs + invoiceDiffs + customerDiffs,
		"updated_at":         time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_reports?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, reportID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) saveReconciliationDiff(diff *tables.ReconciliationDiff) error {
	diffData := map[string]interface{}{
		"id":           diff.ID,
		"report_id":    diff.ReportID,
		"record_type":  diff.RecordType,
		"record_id":    diff.RecordID,
		"user_id":      diff.UserID,
		"diff_type":    diff.DiffType,
		"field_name":   diff.FieldName,
		"local_value":  diff.LocalValue,
		"stripe_value": diff.StripeValue,
		"severity":     diff.Severity,
		"status":       diff.Status,
		"created_at":   diff.CreatedAt,
		"updated_at":   diff.UpdatedAt,
	}

	jsonData, _ := json.Marshal(diffData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs", config.Cfg.Supabase.SupabaseUrl)
	req, _ := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) updateReconciliationDiff(diff *tables.ReconciliationDiff) error {
	updateData := map[string]interface{}{
		"status":     diff.Status,
		"auto_fixed": diff.AutoFixed,
		"fixed_at":   diff.FixedAt,
		"fixed_by":   diff.FixedBy,
		"updated_at": diff.UpdatedAt,
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_diffs?id=eq.%s", config.Cfg.Supabase.SupabaseUrl, diff.ID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) logAuditEvent(reportID, action, recordType, recordID, userID, details string) {
	auditLog := map[string]interface{}{
		"id":           uuid.New().String(),
		"report_id":    reportID,
		"action":       action,
		"record_type":  recordType,
		"record_id":    recordID,
		"user_id":      userID,
		"details":      details,
		"performed_by": "system",
		"created_at":   time.Now(),
	}

	jsonData, _ := json.Marshal(auditLog)
	baseURL := fmt.Sprintf("%s/rest/v1/reconciliation_audit_logs", config.Cfg.Supabase.SupabaseUrl)
	req, _ := http.NewRequest("POST", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	httpclient.Client.Do(req)
}

// 获取本地数据的辅助函数
func (rs *ReconciliationService) getLocalSubscriptions(startDate, endDate time.Time) ([]tables.StripeSubscription, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("created_at", "gte."+startDate.Format(time.RFC3339))
	queryParams.Add("created_at", "lte."+endDate.Format(time.RFC3339))
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var subscriptions []tables.StripeSubscription
	json.NewDecoder(resp.Body).Decode(&subscriptions)
	return subscriptions, nil
}

func (rs *ReconciliationService) getLocalInvoices(startDate, endDate time.Time) ([]tables.StripeInvoice, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_invoices", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("created_at", "gte."+startDate.Format(time.RFC3339))
	queryParams.Add("created_at", "lte."+endDate.Format(time.RFC3339))
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var invoices []tables.StripeInvoice
	json.NewDecoder(resp.Body).Decode(&invoices)
	return invoices, nil
}

func (rs *ReconciliationService) getLocalCustomers(startDate, endDate time.Time) ([]tables.StripeCustomer, error) {
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_customers", config.Cfg.Supabase.SupabaseUrl)
	queryParams := url.Values{}
	queryParams.Add("select", "*")
	queryParams.Add("created_at", "gte."+startDate.Format(time.RFC3339))
	queryParams.Add("created_at", "lte."+endDate.Format(time.RFC3339))
	fullURL := fmt.Sprintf("%s?%s", baseURL, queryParams.Encode())

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var customers []tables.StripeCustomer
	json.NewDecoder(resp.Body).Decode(&customers)
	return customers, nil
}

func (rs *ReconciliationService) calculateMatchedRecords(startDate, endDate time.Time) int {
	// 简单计算匹配记录数（这里可以根据实际需求调整）
	return 100 // 示例值
}

func (rs *ReconciliationService) sendReconciliationNotification(reportID string, diffCount int) {
	// TODO: 实现邮件通知功能
	logger.Logger.Info("发送对账通知", "report_id", reportID, "diff_count", diffCount)
}

func getReconciliationConfig() (*tables.ReconciliationConfig, error) {
	// 默认配置
	return &tables.ReconciliationConfig{
		ID:                  "default",
		Enabled:             true,
		Schedule:            "0 2 * * *", // 每天凌晨2点执行
		AutoFixEnabled:      true,
		AutoFixSeverity:     "medium",
		NotificationEnabled: true,
		NotificationEmail:   "admin@example.com",
		RetentionDays:       30,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil
}

// 修复本地数据的辅助函数
func (rs *ReconciliationService) updateLocalSubscriptionStatus(subscriptionID, status string) error {
	updateData := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_subscriptions?subscription_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, subscriptionID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) updateLocalInvoiceAmount(invoiceID string, amount int) error {
	updateData := map[string]interface{}{
		"amount":     amount,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_invoices?invoice_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, invoiceID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) updateLocalInvoiceStatus(invoiceID, status string) error {
	updateData := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_invoices?invoice_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, invoiceID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (rs *ReconciliationService) updateLocalCustomerEmail(customerID, email string) error {
	updateData := map[string]interface{}{
		"email":      email,
		"updated_at": time.Now(),
	}

	jsonData, _ := json.Marshal(updateData)
	baseURL := fmt.Sprintf("%s/rest/v1/stripe_customers?customer_id=eq.%s", config.Cfg.Supabase.SupabaseUrl, customerID)
	req, _ := http.NewRequest("PATCH", baseURL, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
} 