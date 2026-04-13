CREATE INDEX IF NOT EXISTS idx_products_search
ON products
USING GIN (to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, '')));
