package tables

import (
	"time"
)

// ReconciliationReport 对账报告
type ReconciliationReport struct {
	ID              string    `json:"id"`
	ReportDate      time.Time `json:"report_date"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	Status          string    `json:"status"` // pending, running, completed, failed
	TotalRecords    int       `json:"total_records"`
	MatchedRecords  int       `json:"matched_records"`
	MismatchedRecords int     `json:"mismatched_records"`
	MissingRecords  int       `json:"missing_records"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ReconciliationDiff 对账差异记录
type ReconciliationDiff struct {
	ID                string    `json:"id"`
	ReportID          string    `json:"report_id"`
	RecordType        string    `json:"record_type"` // subscription, invoice, customer
	RecordID          string    `json:"record_id"`
	UserID            string    `json:"user_id"`
	DiffType          string    `json:"diff_type"` // missing, mismatch, extra
	FieldName         string    `json:"field_name"`
	LocalValue        string    `json:"local_value"`
	StripeValue       string    `json:"stripe_value"`
	Severity          string    `json:"severity"` // low, medium, high, critical
	Status            string    `json:"status"` // pending, auto_fixed, manual_fixed, ignored
	AutoFixed         bool      `json:"auto_fixed"`
	FixedAt           *time.Time `json:"fixed_at"`
	FixedBy           string    `json:"fixed_by"`
	Notes             string    `json:"notes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ReconciliationAuditLog 对账审计日志
type ReconciliationAuditLog struct {
	ID              string    `json:"id"`
	ReportID        string    `json:"report_id"`
	Action          string    `json:"action"` // reconciliation_started, diff_found, auto_fix, manual_fix
	RecordType      string    `json:"record_type"`
	RecordID        string    `json:"record_id"`
	UserID          string    `json:"user_id"`
	Details         string    `json:"details"`
	PerformedBy     string    `json:"performed_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// ReconciliationConfig 对账配置
type ReconciliationConfig struct {
	ID                    string    `json:"id"`
	Enabled               bool      `json:"enabled"`
	Schedule              string    `json:"schedule"` // cron expression
	AutoFixEnabled        bool      `json:"auto_fix_enabled"`
	AutoFixSeverity       string    `json:"auto_fix_severity"` // low, medium, high, critical
	NotificationEnabled   bool      `json:"notification_enabled"`
	NotificationEmail     string    `json:"notification_email"`
	RetentionDays         int       `json:"retention_days"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
} 