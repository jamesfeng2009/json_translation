-- 创建对账报告表
CREATE TABLE IF NOT EXISTS reconciliation_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_date TIMESTAMP WITH TIME ZONE NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total_records INTEGER DEFAULT 0,
    matched_records INTEGER DEFAULT 0,
    mismatched_records INTEGER DEFAULT 0,
    missing_records INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建对账差异表
CREATE TABLE IF NOT EXISTS reconciliation_diffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES reconciliation_reports(id) ON DELETE CASCADE,
    record_type VARCHAR(20) NOT NULL,
    record_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255),
    diff_type VARCHAR(20) NOT NULL,
    field_name VARCHAR(100) NOT NULL,
    local_value TEXT,
    stripe_value TEXT,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    auto_fixed BOOLEAN DEFAULT FALSE,
    fixed_at TIMESTAMP WITH TIME ZONE,
    fixed_by VARCHAR(100),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建对账审计日志表
CREATE TABLE IF NOT EXISTS reconciliation_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID REFERENCES reconciliation_reports(id) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    record_type VARCHAR(20),
    record_id VARCHAR(255),
    user_id VARCHAR(255),
    details TEXT,
    performed_by VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建对账配置表
CREATE TABLE IF NOT EXISTS reconciliation_config (
    id VARCHAR(50) PRIMARY KEY DEFAULT 'default',
    enabled BOOLEAN DEFAULT TRUE,
    schedule VARCHAR(100) DEFAULT '0 2 * * *',
    auto_fix_enabled BOOLEAN DEFAULT TRUE,
    auto_fix_severity VARCHAR(20) DEFAULT 'medium',
    notification_enabled BOOLEAN DEFAULT TRUE,
    notification_email VARCHAR(255),
    retention_days INTEGER DEFAULT 30,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_reconciliation_reports_status ON reconciliation_reports(status);
CREATE INDEX IF NOT EXISTS idx_reconciliation_reports_date ON reconciliation_reports(report_date);
CREATE INDEX IF NOT EXISTS idx_reconciliation_diffs_report_id ON reconciliation_diffs(report_id);
CREATE INDEX IF NOT EXISTS idx_reconciliation_diffs_status ON reconciliation_diffs(status);
CREATE INDEX IF NOT EXISTS idx_reconciliation_diffs_severity ON reconciliation_diffs(severity);
CREATE INDEX IF NOT EXISTS idx_reconciliation_diffs_record_type ON reconciliation_diffs(record_type);
CREATE INDEX IF NOT EXISTS idx_reconciliation_audit_logs_report_id ON reconciliation_audit_logs(report_id);
CREATE INDEX IF NOT EXISTS idx_reconciliation_audit_logs_action ON reconciliation_audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_reconciliation_audit_logs_created_at ON reconciliation_audit_logs(created_at);

-- 插入默认配置
INSERT INTO reconciliation_config (id, enabled, schedule, auto_fix_enabled, auto_fix_severity, notification_enabled, notification_email, retention_days)
VALUES ('default', true, '0 2 * * *', true, 'medium', true, 'admin@example.com', 30)
ON CONFLICT (id) DO NOTHING;

-- 创建RLS策略
ALTER TABLE reconciliation_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciliation_diffs ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciliation_audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciliation_config ENABLE ROW LEVEL SECURITY;

-- 对账报告RLS策略（只有管理员可以访问）
CREATE POLICY "Admin can manage reconciliation reports" ON reconciliation_reports
    FOR ALL USING (auth.jwt() ->> 'role' = 'admin');

-- 对账差异RLS策略（只有管理员可以访问）
CREATE POLICY "Admin can manage reconciliation diffs" ON reconciliation_diffs
    FOR ALL USING (auth.jwt() ->> 'role' = 'admin');

-- 对账审计日志RLS策略（只有管理员可以访问）
CREATE POLICY "Admin can manage reconciliation audit logs" ON reconciliation_audit_logs
    FOR ALL USING (auth.jwt() ->> 'role' = 'admin');

-- 对账配置RLS策略（只有管理员可以访问）
CREATE POLICY "Admin can manage reconciliation config" ON reconciliation_config
    FOR ALL USING (auth.jwt() ->> 'role' = 'admin');

-- 创建触发器函数，自动更新updated_at字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为相关表添加触发器
CREATE TRIGGER update_reconciliation_reports_updated_at 
    BEFORE UPDATE ON reconciliation_reports 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reconciliation_diffs_updated_at 
    BEFORE UPDATE ON reconciliation_diffs 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reconciliation_config_updated_at 
    BEFORE UPDATE ON reconciliation_config 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column(); 