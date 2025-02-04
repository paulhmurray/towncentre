-- Create merchants table with automatic timestamp updates
CREATE TABLE IF NOT EXISTS merchants (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    business_name VARCHAR(255) NOT NULL,
    store_name VARCHAR(255),
    store_slug VARCHAR(255),
    region VARCHAR(50),
    description TEXT,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(50),
    business_type VARCHAR(50) NOT NULL,
    location VARCHAR(255),
    opening_hours TEXT,
    password_hash BINARY(60) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Add an index for faster slug lookups
CREATE INDEX IF NOT EXISTS idx_store_slug_region ON merchants(store_slug, region);

CREATE TABLE IF NOT EXISTS sessions (
    token CHAR(43) PRIMARY KEY,
    data BLOB NOT NULL,
    expiry TIMESTAMP(6) NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry);

CREATE TABLE IF NOT EXISTS products (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    merchant_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category VARCHAR(50) NOT NULL,
    image_path VARCHAR(255),
    thumbnail_path VARCHAR(255),
    has_delivery BOOLEAN DEFAULT false,
    has_pickup BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
