-- Platform Database Initialization Script
-- PostgreSQL 15+

-- 创建扩展（如果需要）
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',

    -- 索引
    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_email_key UNIQUE (email)
);

-- 用户名索引（用于快速查询）
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
-- 邮箱索引（用于快速查询）
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
-- 删除时间索引（软删除查询）
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;

-- 代码源表
CREATE TABLE IF NOT EXISTS code_sources (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(10) NOT NULL, -- 'zip', 'jar', 'git', 'path'
    name VARCHAR(255) NOT NULL,
    size BIGINT DEFAULT 0,
    url TEXT,
    file_path TEXT,
    path TEXT,
    status VARCHAR(20) DEFAULT 'uploaded',
    language VARCHAR(50)
);
CREATE INDEX IF NOT EXISTS idx_code_sources_user_id ON code_sources(user_id);
CREATE INDEX IF NOT EXISTS idx_code_sources_status ON code_sources(status);
CREATE INDEX IF NOT EXISTS idx_code_sources_created_at ON code_sources(created_at);

-- 模型配置表
CREATE TABLE IF NOT EXISTS model_configs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL, -- 'openai', 'anthropic', 'azure', etc.
    api_key VARCHAR(500),
    base_url VARCHAR(500),
    model VARCHAR(255),
    max_tokens INTEGER DEFAULT 4000,
    is_active BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active'
);
CREATE INDEX IF NOT EXISTS idx_model_configs_user_id ON model_configs(user_id);
CREATE INDEX IF NOT EXISTS idx_model_configs_is_active ON model_configs(is_active);

-- 任务表
CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_source_id INTEGER REFERENCES code_sources(id) ON DELETE SET NULL,
    model_config_id INTEGER REFERENCES model_configs(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    prompt TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    progress INTEGER DEFAULT 0,
    result TEXT,
    report_path TEXT,
    scanned_files INTEGER DEFAULT 0,
    vulnerability_count INTEGER DEFAULT 0,
    duration INTEGER DEFAULT 0,
    start_time BIGINT,
    end_time BIGINT,
    current_file VARCHAR(500),
    log TEXT,
    detected_language VARCHAR(50),
    ai_log TEXT,
    cross_file_analysis TEXT,
    data_flow_analysis TEXT,
    call_chain_analysis TEXT,
    dependency_analysis TEXT,
    exploit_chain TEXT,
    security_score INTEGER DEFAULT 100,
    risk_level VARCHAR(20),
    code_lines INTEGER DEFAULT 0,
    total_classes INTEGER DEFAULT 0,
    total_functions INTEGER DEFAULT 0,
    critical_vulns INTEGER DEFAULT 0,
    high_vulns INTEGER DEFAULT 0,
    medium_vulns INTEGER DEFAULT 0,
    low_vulns INTEGER DEFAULT 0,
    source_path VARCHAR(500)
);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_code_source_id ON tasks(code_source_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);

-- 漏洞表
CREATE TABLE IF NOT EXISTS vulnerabilities (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    type VARCHAR(100),
    file VARCHAR(500),
    line INTEGER,
    severity VARCHAR(20),
    description TEXT,
    analysis TEXT,
    fix_suggestion TEXT,
    poc TEXT,
    cwe VARCHAR(50),
    cve VARCHAR(50),
    confidence VARCHAR(20),
    attack_vector VARCHAR(50),
    impact TEXT,
    refs TEXT,
    code_snippet TEXT
);
CREATE INDEX IF NOT EXISTS idx_vulnerabilities_task_id ON vulnerabilities(task_id);
CREATE INDEX IF NOT EXISTS idx_vulnerabilities_severity ON vulnerabilities(severity);
CREATE INDEX IF NOT EXISTS idx_vulnerabilities_type ON vulnerabilities(type);

-- 漏洞利用链表
CREATE TABLE IF NOT EXISTS vulnerability_chains (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    name VARCHAR(200),
    description TEXT,
    severity VARCHAR(20),
    total_score DOUBLE PRECISION,
    attack_complexity VARCHAR(50),
    privileges_required VARCHAR(50),
    user_interaction VARCHAR(50),
    scope VARCHAR(50),
    confidentiality VARCHAR(20),
    integrity VARCHAR(20),
    availability VARCHAR(20),
    steps TEXT,
    chain TEXT
);
CREATE INDEX IF NOT EXISTS idx_vulnerability_chains_task_id ON vulnerability_chains(task_id);

-- 项目统计表
CREATE TABLE IF NOT EXISTS project_stats (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    total_files INTEGER DEFAULT 0,
    code_lines INTEGER DEFAULT 0,
    total_classes INTEGER DEFAULT 0,
    total_functions INTEGER DEFAULT 0,
    critical_vulns INTEGER DEFAULT 0,
    high_vulns INTEGER DEFAULT 0,
    medium_vulns INTEGER DEFAULT 0,
    low_vulns INTEGER DEFAULT 0,
    info_vulns INTEGER DEFAULT 0,
    security_score INTEGER DEFAULT 100,
    language VARCHAR(50),
    framework VARCHAR(100),
    dependencies TEXT,
    file_type_distribution TEXT,
    vuln_type_distribution TEXT
);
CREATE INDEX IF NOT EXISTS idx_project_stats_task_id ON project_stats(task_id);

-- 创建更新触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为每个表创建更新触发器
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_code_sources_updated_at BEFORE UPDATE ON code_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_model_configs_updated_at BEFORE UPDATE ON model_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vulnerabilities_updated_at BEFORE UPDATE ON vulnerabilities
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vulnerability_chains_updated_at BEFORE UPDATE ON vulnerability_chains
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_project_stats_updated_at BEFORE UPDATE ON project_stats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 清理函数 - 清理过期记录
CREATE OR REPLACE FUNCTION cleanup_old_records()
RETURNS void AS $$
BEGIN
    -- 清理90天前的软删除记录
    DELETE FROM users WHERE deleted_at < NOW() - INTERVAL '90 days';
    DELETE FROM code_sources WHERE deleted_at < NOW() - INTERVAL '90 days';
    DELETE FROM tasks WHERE deleted_at < NOW() - INTERVAL '90 days';
END;
$$ language 'plpgsql';

-- 插入默认管理员用户 (密码: admin123, 需要在首次登录后更改)
-- 注意：实际密码应该使用bcrypt哈希，这里是示例
INSERT INTO users (username, email, password, role)
VALUES (
    'sYsAdMin',
    'admin@example.com',
    crypt('pAsSwOrd@123!', gen_salt('bf', 10)),
    'admin'
)
ON CONFLICT (username) DO UPDATE
SET password = EXCLUDED.password;

-- 创建性能优化的表空间（可选）
-- COMMENT ON TABLE users IS '用户表 - 存储系统用户信息';
-- COMMENT ON TABLE code_sources IS '代码源表 - 存储上传的代码包';
-- COMMENT ON TABLE model_configs IS '模型配置表 - 存储AI模型配置';
-- COMMENT ON TABLE tasks IS '任务表 - 存储分析任务';
-- COMMENT ON TABLE vulnerabilities IS '漏洞表 - 存储扫描发现的漏洞';
-- COMMENT ON TABLE vulnerability_chains IS '漏洞利用链 - 存储漏洞利用链信息';
-- COMMENT ON TABLE project_stats IS '项目统计 - 存储项目统计信息';
