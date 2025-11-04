
--+ Create the tasks table
CREATE TABLE IF NOT EXISTS tasks (
id SERIAL PRIMARY KEY,
title VARCHAR(255) NOT NULL,
completed BOOLEAN DEFAULT false,
created_at TIMESTAMPTZ DEFAULT NOW()
);

--+ Insert some dummy data
INSERT INTO tasks (title, completed) VALUES ('Learn Go', true);
INSERT INTO tasks (title) VALUES ('Build Observability Project');
INSERT INTO tasks (title) VALUES ('Test Rate Limiting');
