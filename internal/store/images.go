package store

import "context"

func (s *BookStore) SetImage(ctx context.Context, id int64, path string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO book_images(book_id, path) VALUES (?, ?)
		ON CONFLICT(book_id) DO UPDATE SET path = excluded.path`, id, path)
	return err
}

func (s *AuthorStore) SetImage(ctx context.Context, id int64, path string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO author_images(author_id, path) VALUES (?, ?)
		ON CONFLICT(author_id) DO UPDATE SET path = excluded.path`, id, path)
	return err
}
