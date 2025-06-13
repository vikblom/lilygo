package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "embed"

	"github.com/justinas/alice"
)

const (
	height = 540
	width  = 960
)

//go:embed static
var static embed.FS

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
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

func storeImage(r io.Reader) error {
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

	image, err := png.Decode(bytes.NewBuffer(pngBytes))
	if err != nil {
		return fmt.Errorf("png decode: %w", err)
	}
	max := image.Bounds().Max

	for i := range framebuffer {
		framebuffer[i] = 0xFF // White
	}
	for i := range max.X {
		for j := range max.Y {
			_, _, _, a := image.At(i, j).RGBA()
			color := 16 - uint8(a/4096) // Downsize to a nibble, invert black to white.
			if color < 16 {
				drawPixel(i, j, color)
			}
		}
	}

	log.Println("updated image")
	return nil
}

var framebuffer = make([]byte, 960*540/2)

func drawPixel(x, y int, color uint8) {
	if x < 0 || x > width {
		return
	}
	if y < 0 || y > height {
		return
	}
	v := framebuffer[y*width/2+x/2]
	if x%2 > 0 {
		framebuffer[y*width/2+x/2] = (v & 0x0F) | (color << 4)
	} else {
		framebuffer[y*width/2+x/2] = (v & 0xF0) | (color & 0x0F)
	}
}

func handleGetImage(w http.ResponseWriter, r *http.Request) {
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

	n, _ := w.Write(framebuffer[idx*quarterSize : (idx+1)*quarterSize])
	log.Printf("wrote %d\n", n)
}
func handleGetFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("content-type", "image/svg+xml")
	// <link rel="icon" href="data:image/svg+xml">
	w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<text y=".9em" font-size="90">🎨️</text>
</svg>`))
}

func run(ctx context.Context) error {
	log.Println("start")
	for i := range framebuffer {
		framebuffer[i] = 0xFF // White
	}

	fs, err := fs.Sub(static, "static")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("POST /image", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := storeImage(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	mux.Handle("GET /image/{index}", http.HandlerFunc(handleGetImage))
	mux.Handle("GET /favicon.svg", http.HandlerFunc(handleGetFavicon))
	mux.Handle("GET /", http.FileServerFS(fs))

	srv := http.Server{
		Addr: ":8080",
		Handler: alice.New(
			loggingMiddleware,
			contentTypeMiddleware,
		).Then(mux),
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return ctx.Err()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	err := run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
