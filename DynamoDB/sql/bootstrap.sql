CREATE TABLE IF NOT EXISTS _ddb_tables (
    table_name      VARCHAR(255) PRIMARY KEY,
    pk_name         VARCHAR(255) NOT NULL,
    sk_name         VARCHAR(255),
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS _ddb_indexes (
    table_name       VARCHAR(255) NOT NULL,
    index_name       VARCHAR(255) NOT NULL,
    index_type       ENUM('LSI', 'GSI') NOT NULL,
    pk_name          VARCHAR(255) NOT NULL,
    sk_name          VARCHAR(255),
    projection_type  ENUM('ALL', 'KEYS_ONLY', 'INCLUDE') DEFAULT 'ALL',
    projection_attrs TEXT,
    PRIMARY KEY (table_name, index_name)
);
