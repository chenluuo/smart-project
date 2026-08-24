CREATE TABLE IF NOT EXISTS plot_threshold_configs (
    plot_id BIGINT NOT NULL,
    config_version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (plot_id),
    CONSTRAINT fk_plot_threshold_configs_plot FOREIGN KEY (plot_id) REFERENCES plots (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS threshold_config_deliveries (
    id BIGINT NOT NULL AUTO_INCREMENT,
    message_id VARCHAR(64) NOT NULL,
    plot_id BIGINT NOT NULL,
    changed_rule_id BIGINT NOT NULL,
    device_id BIGINT NOT NULL,
    config_version BIGINT UNSIGNED NOT NULL,
    status VARCHAR(16) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    sent_at DATETIME(6) NULL,
    acknowledged_at DATETIME(6) NULL,
    last_error VARCHAR(500) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_threshold_delivery_message UNIQUE (message_id),
    CONSTRAINT uk_threshold_delivery_device_version UNIQUE (plot_id, device_id, config_version),
    KEY idx_threshold_delivery_rule_version (changed_rule_id, config_version),
    KEY idx_threshold_delivery_status_expiry (status, expires_at),
    CONSTRAINT fk_threshold_delivery_plot FOREIGN KEY (plot_id) REFERENCES plots (id),
    CONSTRAINT fk_threshold_delivery_rule FOREIGN KEY (changed_rule_id) REFERENCES alert_rules (id),
    CONSTRAINT fk_threshold_delivery_device FOREIGN KEY (device_id) REFERENCES devices (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
