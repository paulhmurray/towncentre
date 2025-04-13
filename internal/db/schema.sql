-- Merchants table with automatic timestamp updates
CREATE TABLE IF NOT EXISTS merchants (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    business_name VARCHAR(255) NOT NULL,
    store_name VARCHAR(255),
    store_slug VARCHAR(255),
    logo_path VARCHAR(255),
    region VARCHAR(50),
    description TEXT,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(50),
    business_type VARCHAR(50) NOT NULL,
    business_model ENUM('product', 'service', 'hybrid') NOT NULL DEFAULT 'product',
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


CREATE TABLE IF NOT EXISTS messages (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    merchant_id BIGINT UNSIGNED NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(50),
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT false,
    status ENUM('pending', 'approved', 'rejected') DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (merchant_id) REFERENCES merchants(id),
    CHECK (LENGTH(message) <= 1000) -- Limit message length
);
CREATE TABLE IF NOT EXISTS store_views (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    merchant_id BIGINT UNSIGNED NOT NULL,
    viewed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    viewer_ip VARCHAR(45),  -- IPv6 addresses can be up to 45 chars
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    merchant_id BIGINT UNSIGNED NOT NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Index for faster token lookups
CREATE INDEX IF NOT EXISTS idx_reset_token ON password_reset_tokens(token);

-- Create services table
CREATE TABLE IF NOT EXISTS services (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    merchant_id BIGINT UNSIGNED NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100) NOT NULL,
    price_type ENUM('fixed', 'hourly', 'quote', 'range', 'free') NOT NULL,
    price_from DECIMAL(10,2),
    price_to DECIMAL(10,2),
    availability TEXT,
    service_area TEXT,
    is_featured BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
);

-- Create service_categories table
CREATE TABLE IF NOT EXISTS service_categories (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    icon VARCHAR(50),
    display_order INT DEFAULT 0
);

-- Insert common service categories
INSERT INTO service_categories (name, slug, icon, display_order) VALUES
('Plumbing', 'plumbing', 'pipe', 10),
('Electrical', 'electrical', 'lightning', 20),
('Cleaning', 'cleaning', 'sparkles', 30),
('Gardening', 'gardening', 'flower', 40),
('Home Maintenance', 'home-maintenance', 'house', 50),
('Consulting', 'consulting', 'briefcase', 60),
('Health & Wellness', 'health-wellness', 'heartbeat', 70),
('Education & Tutoring', 'education', 'book', 80),
('Legal', 'legal', 'scale', 90),
('Automotive', 'automotive', 'car', 100);

-- Modify merchants table to support service businesses
ALTER TABLE merchants 
ADD COLUMN business_type_detail VARCHAR(100) AFTER business_type,
ADD COLUMN service_areas TEXT AFTER location,
ADD COLUMN qualifications TEXT AFTER service_areas,
ADD COLUMN years_experience INT AFTER qualifications,
ADD COLUMN license_info VARCHAR(255) AFTER years_experience;
