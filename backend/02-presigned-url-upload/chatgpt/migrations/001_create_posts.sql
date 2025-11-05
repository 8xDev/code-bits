-- create posts table
CREATE TABLE IF NOT EXISTS posts (
  id SERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT,
  object_key TEXT NOT NULL,
  media_url TEXT NOT NULL, -- constructed URL for convenience
  media_type TEXT NOT NULL CHECK (media_type IN ('image','video')),
  size_bytes BIGINT DEFAULT 0,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
