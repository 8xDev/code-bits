-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
  id SERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  content TEXT,
  done BOOLEAN DEFAULT false,
  created_at TIMESTAMP DEFAULT now()
);

-- seed
INSERT INTO tasks (title, content) VALUES
('Write docs', 'Create README and architecture docs'),
('Buy milk', 'Remember milk for breakfast');
