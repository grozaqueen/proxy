CREATE TABLE IF NOT EXISTS requests (
                                        id TEXT PRIMARY KEY,
                                        method TEXT,
                                        path TEXT,
                                        scheme TEXT,
                                        host TEXT,
                                        get_params JSONB,
                                        headers JSONB,
                                        cookies JSONB,
                                        post_params JSONB,
                                        raw_body BYTEA,
                                        response_code INTEGER DEFAULT NULL,
                                        response_message TEXT,
                                        response_headers JSONB,
                                        response_body BYTEA,
                                        timestamp TIMESTAMP
);

CREATE INDEX IF NOT EXISTS requests_timestamp_idx ON requests (timestamp);