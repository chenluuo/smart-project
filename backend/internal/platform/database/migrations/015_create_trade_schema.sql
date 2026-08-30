CREATE TABLE warehouses (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    code VARCHAR(64) NOT NULL,
    remark VARCHAR(255) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_warehouses_code UNIQUE (code)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE materials (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(64) NOT NULL,
    unit VARCHAR(32) NOT NULL,
    spec VARCHAR(255) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_materials_category (category)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE stocks (
    id BIGINT NOT NULL AUTO_INCREMENT,
    warehouse_id BIGINT NOT NULL,
    material_id BIGINT NOT NULL,
    quantity DECIMAL(18,3) NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_stocks_warehouse_material UNIQUE (warehouse_id, material_id),
    KEY idx_stocks_material (material_id),
    CONSTRAINT fk_stocks_warehouse FOREIGN KEY (warehouse_id) REFERENCES warehouses (id),
    CONSTRAINT fk_stocks_material FOREIGN KEY (material_id) REFERENCES materials (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE order_headers (
    id BIGINT NOT NULL AUTO_INCREMENT,
    order_no VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/APPROVED/TRADING/CONFIRMED/CLOSED/REJECTED/DELETED',
    customer_id BIGINT NOT NULL,
    expected_time DATETIME(6) NULL,
    remark VARCHAR(255) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT uk_order_headers_no UNIQUE (order_no),
    KEY idx_order_headers_customer (customer_id),
    KEY idx_order_headers_status (status),
    CONSTRAINT fk_order_headers_customer FOREIGN KEY (customer_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE order_items (
    id BIGINT NOT NULL AUTO_INCREMENT,
    order_id BIGINT NOT NULL,
    material_id BIGINT NOT NULL,
    quantity DECIMAL(18,3) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_order_items_order (order_id),
    KEY idx_order_items_material (material_id),
    CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES order_headers (id),
    CONSTRAINT fk_order_items_material FOREIGN KEY (material_id) REFERENCES materials (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE stock_records (
    id BIGINT NOT NULL AUTO_INCREMENT,
    warehouse_id BIGINT NOT NULL,
    material_id BIGINT NOT NULL,
    direction VARCHAR(16) NOT NULL COMMENT 'IN/OUT',
    quantity DECIMAL(18,3) NOT NULL,
    order_id BIGINT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    KEY idx_stock_records_warehouse (warehouse_id),
    KEY idx_stock_records_material (material_id),
    KEY idx_stock_records_order (order_id),
    CONSTRAINT fk_stock_records_warehouse FOREIGN KEY (warehouse_id) REFERENCES warehouses (id),
    CONSTRAINT fk_stock_records_material FOREIGN KEY (material_id) REFERENCES materials (id),
    CONSTRAINT fk_stock_records_order FOREIGN KEY (order_id) REFERENCES order_headers (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

INSERT IGNORE INTO roles (role_code, role_name) VALUES ('CUSTOMER', '顾客'), ('WAREHOUSE_MANAGER', '仓库管理员');
