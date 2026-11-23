CREATE TABLE products (
                          id          UUID PRIMARY KEY DEFAULT uuidv7(),
                          name        TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
                          description TEXT NOT NULL DEFAULT '',
                          price_minor BIGINT NOT NULL CHECK (price_minor >= 0),
                          currency    CHAR(3) NOT NULL CHECK (currency IN ('USD', 'EUR', 'UAH')),
                          attributes  JSONB NOT NULL DEFAULT '{}',
                          created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
                          updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- keyset pagination, newest first
CREATE INDEX products_created_at_id_idx ON products (created_at DESC, id DESC);