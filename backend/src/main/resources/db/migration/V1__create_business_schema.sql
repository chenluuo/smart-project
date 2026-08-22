CREATE TABLE users (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL,
    mobile VARCHAR(32) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_users_mobile UNIQUE (mobile)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE roles (
    id BIGINT NOT NULL AUTO_INCREMENT,
    role_code VARCHAR(64) NOT NULL,
    role_name VARCHAR(64) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_roles_code UNIQUE (role_code)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES roles (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE farms (
    id BIGINT NOT NULL AUTO_INCREMENT,
    owner_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    address VARCHAR(255) NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_farms_owner (owner_id),
    CONSTRAINT fk_farms_owner FOREIGN KEY (owner_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE farm_users (
    id BIGINT NOT NULL AUTO_INCREMENT,
    farm_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    farm_role VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_farm_user UNIQUE (farm_id, user_id),
    KEY idx_farm_users_user (user_id),
    CONSTRAINT fk_farm_users_farm FOREIGN KEY (farm_id) REFERENCES farms (id),
    CONSTRAINT fk_farm_users_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE plots (
    id BIGINT NOT NULL AUTO_INCREMENT,
    farm_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    crop_type VARCHAR(64) NULL,
    growth_stage VARCHAR(64) NULL,
    area DECIMAL(12, 2) NULL,
    location VARCHAR(255) NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_plots_farm (farm_id),
    CONSTRAINT fk_plots_farm FOREIGN KEY (farm_id) REFERENCES farms (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE devices (
    id BIGINT NOT NULL AUTO_INCREMENT,
    device_code VARCHAR(64) NOT NULL,
    serial_no VARCHAR(128) NOT NULL,
    device_type VARCHAR(64) NOT NULL,
    model VARCHAR(64) NULL,
    status VARCHAR(32) NOT NULL,
    credential_status VARCHAR(32) NOT NULL,
    activated_at DATETIME(6) NULL,
    last_seen_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_devices_code UNIQUE (device_code),
    CONSTRAINT uk_devices_serial UNIQUE (serial_no),
    KEY idx_devices_status_last_seen (status, last_seen_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE device_bindings (
    id BIGINT NOT NULL AUTO_INCREMENT,
    device_id BIGINT NOT NULL,
    plot_id BIGINT NOT NULL,
    bound_by BIGINT NOT NULL,
    bound_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    unbound_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    KEY idx_device_bindings_device_active (device_id, unbound_at),
    KEY idx_device_bindings_plot_active (plot_id, unbound_at),
    KEY idx_device_bindings_bound_by (bound_by),
    CONSTRAINT fk_device_bindings_device FOREIGN KEY (device_id) REFERENCES devices (id),
    CONSTRAINT fk_device_bindings_plot FOREIGN KEY (plot_id) REFERENCES plots (id),
    CONSTRAINT fk_device_bindings_user FOREIGN KEY (bound_by) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE alert_rules (
    id BIGINT NOT NULL AUTO_INCREMENT,
    plot_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    metric VARCHAR(64) NOT NULL,
    comparison_operator VARCHAR(16) NOT NULL,
    threshold DECIMAL(14, 4) NOT NULL,
    duration_seconds INT NOT NULL,
    hysteresis DECIMAL(14, 4) NOT NULL DEFAULT 0,
    level VARCHAR(16) NOT NULL,
    enabled BIT(1) NOT NULL DEFAULT b'1',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_alert_rules_plot_metric_enabled (plot_id, metric, enabled),
    CONSTRAINT fk_alert_rules_plot FOREIGN KEY (plot_id) REFERENCES plots (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE alerts (
    id BIGINT NOT NULL AUTO_INCREMENT,
    rule_id BIGINT NOT NULL,
    device_id BIGINT NULL,
    acknowledged_by BIGINT NULL,
    level VARCHAR(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    trigger_value DECIMAL(14, 4) NOT NULL,
    triggered_at DATETIME(6) NOT NULL,
    acknowledged_at DATETIME(6) NULL,
    resolved_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_alerts_rule_status (rule_id, status),
    KEY idx_alerts_device_triggered (device_id, triggered_at),
    KEY idx_alerts_ack_user (acknowledged_by),
    CONSTRAINT fk_alerts_rule FOREIGN KEY (rule_id) REFERENCES alert_rules (id),
    CONSTRAINT fk_alerts_device FOREIGN KEY (device_id) REFERENCES devices (id),
    CONSTRAINT fk_alerts_ack_user FOREIGN KEY (acknowledged_by) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE device_commands (
    id BIGINT NOT NULL AUTO_INCREMENT,
    command_id VARCHAR(64) NOT NULL,
    device_id BIGINT NOT NULL,
    plot_id BIGINT NOT NULL,
    issued_by BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    parameters_json JSON NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_code VARCHAR(64) NULL,
    error_message VARCHAR(500) NULL,
    issued_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    executed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_device_commands_command_id UNIQUE (command_id),
    CONSTRAINT uk_device_commands_idempotency UNIQUE (idempotency_key),
    KEY idx_device_commands_device_status (device_id, status),
    KEY idx_device_commands_plot_created (plot_id, created_at),
    KEY idx_device_commands_issuer (issued_by),
    CONSTRAINT fk_device_commands_device FOREIGN KEY (device_id) REFERENCES devices (id),
    CONSTRAINT fk_device_commands_plot FOREIGN KEY (plot_id) REFERENCES plots (id),
    CONSTRAINT fk_device_commands_user FOREIGN KEY (issued_by) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE notifications (
    id BIGINT NOT NULL AUTO_INCREMENT,
    alert_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    channel VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    sent_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_notifications_status_created (status, created_at),
    KEY idx_notifications_user_created (user_id, created_at),
    KEY idx_notifications_alert (alert_id),
    CONSTRAINT fk_notifications_alert FOREIGN KEY (alert_id) REFERENCES alerts (id),
    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE audit_logs (
    id BIGINT NOT NULL AUTO_INCREMENT,
    farm_id BIGINT NULL,
    actor_id BIGINT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NULL,
    result VARCHAR(32) NOT NULL,
    request_id VARCHAR(64) NULL,
    trace_id VARCHAR(64) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_audit_logs_farm_created (farm_id, created_at),
    KEY idx_audit_logs_actor_created (actor_id, created_at),
    KEY idx_audit_logs_trace (trace_id),
    CONSTRAINT fk_audit_logs_farm FOREIGN KEY (farm_id) REFERENCES farms (id),
    CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE outbox_events (
    id BIGINT NOT NULL AUTO_INCREMENT,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSON NOT NULL,
    status VARCHAR(32) NOT NULL,
    available_at DATETIME(6) NOT NULL,
    published_at DATETIME(6) NULL,
    retry_count INT NOT NULL DEFAULT 0,
    last_error VARCHAR(1000) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_outbox_status_available (status, available_at, id),
    KEY idx_outbox_aggregate (aggregate_type, aggregate_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
