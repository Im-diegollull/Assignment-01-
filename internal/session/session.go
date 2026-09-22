package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
	secure bool
}

func Open(ctx context.Context, addr string, secure bool) (*Store, error) {
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 1})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &Store{client: client, secure: secure}, nil
}

func (s *Store) Close() error { return s.client.Close() }

func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bookID, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/books/"), 10, 64)
		isBook := strings.HasPrefix(r.URL.Path, "/books/") && bookID > 0
		if r.Method != http.MethodGet || (!isBook && r.URL.Path != "/recent" && r.URL.Path != "/session") {
			next.ServeHTTP(w, r)
			return
		}
		id := ""
		if cookie, err := r.Cookie("bookreviews_session"); err == nil {
			if raw, err := hex.DecodeString(cookie.Value); err == nil && len(raw) == 32 {
				id = cookie.Value
			}
		}
		if id == "" {
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				http.Error(w, "Session unavailable", http.StatusInternalServerError)
				return
			}
			id = hex.EncodeToString(raw)
		}
		key := "session:" + id
		pipe := s.client.TxPipeline()
		pipe.HSet(r.Context(), key, "active", "1")
		if isBook {
			pipe.HSet(r.Context(), key, "last_book", strconv.FormatInt(bookID, 10))
		}
		pipe.Expire(r.Context(), key, 24*time.Hour)
		state := pipe.HGetAll(r.Context(), key)
		if _, err := pipe.Exec(r.Context()); err != nil {
			http.Error(w, "Session unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "bookreviews_session", Value: id, Path: "/", MaxAge: 86400, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
		w.Header().Set("Cache-Control", "private, no-store")
		if r.URL.Path == "/session" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(state.Val())
			return
		}
		if r.URL.Path == "/recent" {
			path := "/books"
			if last, err := strconv.ParseInt(state.Val()["last_book"], 10, 64); err == nil && last > 0 {
				path += "/" + strconv.FormatInt(last, 10)
			}
			http.Redirect(w, r, path, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
