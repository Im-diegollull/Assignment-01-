package handlers

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bookreviews/internal/database"
	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

func TestImagesAcrossInstances(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	author := models.Author{Name: "Author"}
	if err := st.Authors.Create(ctx, &author); err != nil {
		t.Fatal(err)
	}
	book := models.Book{Name: "Book", AuthorID: author.ID}
	if err := st.Books.Create(ctx, &book); err != nil {
		t.Fatal(err)
	}
	newInstance := func(proxy bool) http.Handler {
		t.Helper()
		other, err := database.Open(ctx, filepath.Join(root, "app.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { other.Close() })
		h, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store.New(other), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		h.ConfigureFiles(filepath.Join(root, "media"), proxy)
		return h.Routes()
	}
	first, second := newInstance(false), newInstance(false)
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	upload := func(route string, raw []byte) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("Image", "../../unsafe.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", route, &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		first.ServeHTTP(rec, req)
		return rec
	}
	for _, route := range []string{"/books/1/image", "/authors/1/image"} {
		if rec := upload(route, picture.Bytes()); rec.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d %s", route, rec.Code, rec.Body.String())
		}
		if rec := upload(route, []byte("<script>bad</script>")); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid image: %d", rec.Code)
		}
	}
	saved, err := st.Books.Get(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ImagePath == "" || filepath.Base(saved.ImagePath) != saved.ImagePath {
		t.Fatal("unsafe or missing image path")
	}
	storedAuthor, err := st.Authors.Get(ctx, author.ID)
	if err != nil || storedAuthor.ImagePath == "" {
		t.Fatalf("author image: %v", err)
	}
	rec := httptest.NewRecorder()
	second.ServeHTTP(rec, httptest.NewRequest("GET", "/media/"+saved.ImagePath, nil))
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), picture.Bytes()) {
		t.Fatalf("shared image: %d", rec.Code)
	}
	proxy := newInstance(true)
	for _, route := range []string{"/media/" + saved.ImagePath, "/static/style.css"} {
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, httptest.NewRequest("GET", route, nil))
		if rec.Code != 404 {
			t.Fatalf("proxy mode %s: %d", route, rec.Code)
		}
	}
	if rec := upload("/books/999/image", picture.Bytes()); rec.Code != 404 {
		t.Fatalf("missing book: %d", rec.Code)
	}
}
