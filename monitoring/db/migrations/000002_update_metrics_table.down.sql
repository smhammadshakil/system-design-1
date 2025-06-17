-- Drop composite primary key
ALTER TABLE metrics DROP CONSTRAINT metrics_pkey;

-- Drop timestamp column
ALTER TABLE metrics DROP COLUMN timestamp;

-- Rename ip column back to key
ALTER TABLE metrics RENAME COLUMN ip TO key;

-- Add id column
ALTER TABLE metrics ADD COLUMN id SERIAL;

-- Add primary key constraint
ALTER TABLE metrics ADD PRIMARY KEY (id); 