package api

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
	"github.com/justinas/alice"
	"github.com/vikblom/lilygo/pkg/db"
	"golang.org/x/time/rate"

	_ "embed"
)

const (
	width  = 960 // X coordinate on display (landscape).
	height = 540 // X coordinate in browser (portrait).
)

//go:embed static
var static embed.FS

type Server struct {
	db *db.DB
}

func New(db *db.DB) (http.Handler, error) {
	s := &Server{
		db: db,
	}

	fs, err := fs.Sub(static, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	// Device API endpoints.
	mux.HandleFunc("GET /image", s.handlePickImage)
	mux.HandleFunc("GET /image/{id}/{index}", s.handleGetImage)

	// Web API endpoints.
	mux.HandleFunc("POST /image", s.handleStoreImage)
	mux.HandleFunc("GET /favicon.svg", s.handleGetFavicon)
	mux.Handle("GET /", http.FileServerFS(fs))
	mux.HandleFunc("GET /images", s.handleListImages)
	mux.HandleFunc("GET /images/{id}", s.handleSpecificImage)

	return alice.New(
		loggingMiddleware,
		limitMiddleware,
		contentTypeMiddleware,
	).Then(mux), nil
}

func limitMiddleware(next http.Handler) http.Handler {
	var limiter = rate.NewLimiter(3, 10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limiter.Allow() == false {
			http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(next, w, r)

		if r.Method == "GET" && m.Code == 200 {
			return
		}

		slog.Info(fmt.Sprintf("[%d] %s %s", m.Code, r.Method, r.URL.Path),
			"user_agent", r.UserAgent(),
			"content_length", r.ContentLength,
			"method", r.Method,
			"path", r.URL.Path,
			"client_ip", clientIP(r),
			"status", m.Code,
			"duration", m.Duration,
			"bytes_writter", m.Written,
		)
	})
}

func clientIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

// contentTypeMiddleware injects the right content type.
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/style.css" {
			w.Header().Add("content-type", "text/css; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGetFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-type", "image/svg+xml")
	// <link rel="icon" href="data:image/svg+xml">
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<text y=".9em" font-size="90">🎨️</text>
</svg>`))
}

var storeLimiter = rate.NewLimiter(1, 1)

func (s *Server) handleStoreImage(w http.ResponseWriter, r *http.Request) {
	if storeLimiter.Allow() == false {
		http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
		return
	}

	err := s.storeImage(r.Context(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) storeImage(ctx context.Context, r io.Reader) error {
	bs, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read all: %w", err)
	}

	head, tail, ok := bytes.Cut(bs, []byte(","))
	if !ok {
		return fmt.Errorf("not an image")
	}
	if !bytes.Equal(head, []byte("data:image/png;base64")) {
		return fmt.Errorf("cannot handle header: %q", head)
	}
	pngBytes, err := base64.StdEncoding.AppendDecode(nil, tail)
	if err != nil {
		return fmt.Errorf("b64 decode: %w", err)
	}

	_, err = s.db.AddImage(ctx, pngBytes)
	if err != nil {
		return fmt.Errorf("store image: %w", err)
	}

	return nil
}

var listTemplate = template.Must(template.New("list").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <link rel="stylesheet" href="style.css">
  <link rel="icon" href="favicon.svg">
</head>
<body>
<ul>
  {{range $v := .}}
    <li><a href=/images/{{$v}}>{{ $v }}</a></li>
  {{end}}
</ul>
</body>
</html>
`))

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	ids, err := s.db.ListImages(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(r.Header.Get("accept"), "text/html") {
		err = listTemplate.Execute(w, ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// TODO: Accept json.
		for _, v := range ids {
			fmt.Fprintf(w, "%s\n", v)
		}
	}

}

func (s *Server) handleSpecificImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	png, err := s.db.ReadImage(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("content-type", "image/png")
	_, _ = w.Write(png)
}

func (s *Server) handlePickImage(w http.ResponseWriter, r *http.Request) {
	id, err := s.db.RandomImage(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(id.String()))
}

func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if idx < 0 || 4 <= idx {
		http.Error(w, fmt.Sprintf("idx %d out of bounds [0,4)", idx), http.StatusInternalServerError)
		return
	}

	// Write one quarter at a time.
	quarterSize := 960 * 540 / 2 / 4
	var framebuffer = make([]byte, 960*540/2)
	for i := range framebuffer {
		framebuffer[i] = 0xFF // White
	}

	bs, err := s.db.ReadImage(r.Context(), id)
	// if errors.Is(err, sql.ErrNoRows) {
	// 	_, _ = w.Write(framebuffer[idx*quarterSize : (idx+1)*quarterSize])
	// 	return
	// }
	if err != nil {
		http.Error(w, fmt.Sprintf("png decode: %s", err), http.StatusInternalServerError)
		return
	}

	image, err := png.Decode(bytes.NewBuffer(bs))
	if err != nil {
		http.Error(w, fmt.Sprintf("png decode: %s", err), http.StatusInternalServerError)
		return
	}
	max := image.Bounds().Max

	for i := range framebuffer {
		framebuffer[i] = 0xFF // White
	}
	// The web input is transposed, y is the "long" dimension.
	for i := range min(540, max.X) {
		for j := range min(960, max.Y) {
			_, _, _, a := image.At(i, j).RGBA()
			color := 16 - uint8(a/4096) // Downsize to a nibble, invert black to white.
			if color < 16 {
				drawPixel(framebuffer, j, height-1-i, color) // Mirror after transposing to preserve asymmetries.
			}
		}
	}

	_, _ = w.Write(framebuffer[idx*quarterSize : (idx+1)*quarterSize])
}

func drawPixel(bs []byte, x, y int, color uint8) {
	if x < 0 || x > width {
		return
	}
	if y < 0 || y > height {
		return
	}
	v := bs[y*width/2+x/2]
	if x%2 > 0 {
		bs[y*width/2+x/2] = (v & 0x0F) | (color << 4)
	} else {
		bs[y*width/2+x/2] = (v & 0xF0) | (color & 0x0F)
	}
}
