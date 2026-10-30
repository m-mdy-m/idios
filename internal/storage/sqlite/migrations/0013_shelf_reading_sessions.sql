CREATE TABLE shelf_reading_sessions (
  id TEXT PRIMARY KEY,
  -- Keep the historical book ID even if its source folder disappears from the current library.
  book_id TEXT NOT NULL,
  book_title TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at TEXT NOT NULL,
  duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
  start_page INTEGER NOT NULL DEFAULT 1 CHECK (start_page >= 0),
  end_page INTEGER NOT NULL DEFAULT 1 CHECK (end_page >= 0),
  created_at TEXT NOT NULL
);
CREATE INDEX shelf_reading_sessions_started_at_idx ON shelf_reading_sessions(started_at DESC);
CREATE INDEX shelf_reading_sessions_book_started_idx ON shelf_reading_sessions(book_id, started_at DESC);
