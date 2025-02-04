-- Add new columns to merchants table
ALTER TABLE merchants
ADD COLUMN IF NOT EXISTS store_name VARCHAR(255) AFTER business_name,
ADD COLUMN IF NOT EXISTS store_slug VARCHAR(255) AFTER store_name,
ADD COLUMN IF NOT EXISTS region VARCHAR(50) AFTER store_slug,
ADD COLUMN IF NOT EXISTS description TEXT AFTER region,
ADD COLUMN IF NOT EXISTS location VARCHAR(255) AFTER description,
ADD COLUMN IF NOT EXISTS opening_hours TEXT AFTER location;

-- Add an index for faster slug lookups
CREATE INDEX IF NOT EXISTS idx_store_slug_region ON merchants(store_slug, region);

-- Update existing records to have store_name, store_slug and region
UPDATE merchants 
SET store_name = business_name,
    store_slug = LOWER(REGEXP_REPLACE(REGEXP_REPLACE(business_name, '[^a-zA-Z0-9]+', '-'), '^-+|-+$', '')),
    region = 'ballarat'
WHERE store_slug IS NULL;
