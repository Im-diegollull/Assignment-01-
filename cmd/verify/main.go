package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	base := flag.String("url", "https://app.localhost", "base URL")
	ca := flag.String("ca", "", "local CA certificate")
	instances := flag.Int("instances", 3, "expected application instances")
	sessions := flag.Bool("sessions", true, "verify Redis sessions")
	proxy := flag.Bool("proxy", true, "verify Caddy static serving")
	flag.Parse()
	if err := run(*base, *ca, *instances, *sessions, *proxy); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(base, ca string, instances int, sessions, proxy bool) error {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	if ca != "" {
		raw, err := os.ReadFile(ca)
		if err != nil {
			return err
		}
		if !pool.AppendCertsFromPEM(raw) {
			return fmt.Errorf("invalid CA")
		}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout:       15 * time.Second,
		Jar:           jar,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	request := func(method, path, contentType string, body io.Reader, want int) (http.Header, []byte, error) {
		req, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, body)
		if err != nil {
			return nil, nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, nil, err
		}
		if res.StatusCode != want {
			return nil, nil, fmt.Errorf("%s %s: got %d, want %d: %s", method, path, res.StatusCode, want, raw)
		}
		return res.Header, raw, nil
	}
	seen := map[string]bool{}
	for i := 0; i < 30; i++ {
		h, _, err := request("GET", "/healthz", "", nil, 200)
		if err != nil {
			return err
		}
		if h.Get("X-App-Instance") == "" {
			return fmt.Errorf("missing instance header")
		}
		seen[h.Get("X-App-Instance")] = true
	}
	if len(seen) != instances {
		return fmt.Errorf("saw %d instances, expected %d: %v", len(seen), instances, seen)
	}
	h, _, err := request("GET", "/static/style.css", "", nil, 200)
	if err != nil {
		return err
	}
	if proxy && (h.Get("X-App-Instance") != "" || !strings.Contains(h.Get("Cache-Control"), "max-age")) {
		return fmt.Errorf("static files did not come from edge with cache headers")
	}
	for _, path := range []string{"/books/top-selling", "/authors/stats", "/search?q=isla", "/books/1"} {
		if _, _, err := request("GET", path, "", nil, 200); err != nil {
			return err
		}
	}
	var picture bytes.Buffer
	bitmap := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			bitmap.SetRGBA(x, y, color.RGBA{R: 40, G: 140, B: 90, A: 255})
		}
	}
	if err := png.Encode(&picture, bitmap); err != nil {
		return err
	}
	var mediaPath string
	for _, entity := range []string{"books", "authors"} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("Image", "verification.png")
		if err != nil {
			return err
		}
		if _, err := part.Write(picture.Bytes()); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		if _, _, err := request("POST", "/"+entity+"/1/image", writer.FormDataContentType(), &body, 303); err != nil {
			return err
		}
		_, html, err := request("GET", "/"+entity+"/1", "", nil, 200)
		if err != nil {
			return err
		}
		_, after, ok := strings.Cut(string(html), `src="/media/`)
		if !ok {
			return fmt.Errorf("uploaded image missing from %s", entity)
		}
		name, _, _ := strings.Cut(after, `"`)
		mediaPath = "/media/" + name
		h, raw, err := request("GET", mediaPath, "", nil, 200)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, picture.Bytes()) {
			return fmt.Errorf("uploaded image differs")
		}
		if proxy && (h.Get("X-App-Instance") != "" || !strings.Contains(h.Get("Cache-Control"), "immutable")) {
			return fmt.Errorf("image not served by edge")
		}
	}
	if sessions {
		for i := 0; i < 9; i++ {
			_, raw, err := request("GET", "/session", "", nil, 200)
			if err != nil {
				return err
			}
			var state map[string]string
			if err := json.Unmarshal(raw, &state); err != nil {
				return err
			}
			if state["last_book"] != "1" {
				return fmt.Errorf("lost shared session: %s", raw)
			}
		}
		h, _, err := request("GET", "/recent", "", nil, 303)
		if err != nil {
			return err
		}
		if h.Get("Location") != "/books/1" {
			return fmt.Errorf("lost last book")
		}
		u, _ := url.Parse(base)
		if len(jar.Cookies(u)) == 0 {
			return fmt.Errorf("session cookie missing")
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"url": base, "instances": seen, "sessions": sessions, "media_path": mediaPath, "verified_at": time.Now().UTC()})
}
