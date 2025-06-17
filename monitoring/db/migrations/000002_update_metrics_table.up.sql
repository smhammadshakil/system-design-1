-- Drop existing primary key constraint
ALTER TABLE metrics DROP CONSTRAINT metrics_pkey;

-- Drop the id column
ALTER TABLE metrics DROP COLUMN id;

-- Rename key column to ip
ALTER TABLE metrics RENAME COLUMN key TO ip;

-- Add timestamp column
ALTER TABLE metrics ADD COLUMN timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Add composite primary key
ALTER TABLE metrics ADD PRIMARY KEY (ip, timestamp); 