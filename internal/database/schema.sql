-- Esquema de la base de datos. Se aplica con `go run ./cmd/server -migrate`.
--
-- Todo el DDL es idempotente (IF NOT EXISTS) para poder ejecutarlo en cada
-- arranque sin efectos secundarios.
--
-- Nota sobre fechas: SQLite no tiene tipo DATE, así que se guardan como TEXT en
-- formato ISO-8601 'YYYY-MM-DD'. Ese formato ordena correctamente de forma
-- lexicográfica y es el que entiende strftime().

CREATE TABLE IF NOT EXISTS authors (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT NOT NULL,
  date_of_birth     TEXT,                 -- ISO 'YYYY-MM-DD'
  country_of_origin TEXT,
  description       TEXT
);

CREATE TABLE IF NOT EXISTS books (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  author_id        INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  summary          TEXT,
  publication_date TEXT,                  -- ISO 'YYYY-MM-DD'
  
  number_of_sales  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS reviews (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  review  TEXT,
  score   INTEGER NOT NULL CHECK (score BETWEEN 1 AND 5),
  upvotes INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sales_by_year (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  year    INTEGER NOT NULL,
  sales   INTEGER NOT NULL DEFAULT 0,
  UNIQUE (book_id, year)
);

-- Índices sobre las FK: SQLite no los crea automáticamente, y sin ellos tanto
-- los JOIN de las tablas de §5 como el ON DELETE CASCADE hacen scan completo.
CREATE INDEX IF NOT EXISTS idx_books_author ON books(author_id);
CREATE INDEX IF NOT EXISTS idx_reviews_book ON reviews(book_id);
CREATE INDEX IF NOT EXISTS idx_sales_book  ON sales_by_year(book_id);
CREATE INDEX IF NOT EXISTS idx_sales_year  ON sales_by_year(year);
