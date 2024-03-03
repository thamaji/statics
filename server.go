package statics

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Option func(*option)

func WithCompressibleContentTypes(types []string) Option {
	return func(option *option) {
		option.compressibleContentTypes = types
		sort.Strings(option.compressibleContentTypes)
	}
}

func WithCompressibleContentLength(length int) Option {
	return func(option *option) {
		option.compressibleContentLength = length
	}
}

func WithNotFound(handler http.Handler) Option {
	return func(option *option) {
		option.notFound = handler
	}
}

type option struct {
	compressibleContentTypes  []string
	compressibleContentLength int
	notFound                  http.Handler
}

func FileServer(dir string, options ...Option) http.Handler {
	option := option{
		compressibleContentTypes: []string{
			"application/atom+xml",
			"application/javascript",
			"application/json",
			"application/rss+xml",
			"application/x-javascript",
			"image/svg+xml",
			"text/css",
			"text/html",
			"text/javascript",
			"text/plain",
		},
		compressibleContentLength: 1024,
		notFound:                  http.HandlerFunc(http.NotFound),
	}
	for _, fn := range options {
		fn(&option)
	}

	contents := map[string]http.Handler{}

	filepath.Walk(dir, func(fpath string, fi fs.FileInfo, err error) error {
		relpath, err := filepath.Rel(dir, fpath)
		if err != nil {
			return nil
		}

		if fi.IsDir() {
			return nil
		}

		upath := path.Join("/", filepath.ToSlash(relpath))
		name := fi.Name()
		modtime := fi.ModTime()

		body, err := os.ReadFile(fpath)
		if err != nil {
			return nil
		}

		// detect Content-Type
		contentType := mime.TypeByExtension(filepath.Ext(fpath))
		if contentType == "" {
			if len(body) > 512 {
				contentType = http.DetectContentType(body[:512])
			} else {
				contentType = http.DetectContentType(body)
			}
		}

		var handler http.HandlerFunc
		if len(body) >= option.compressibleContentLength && contentType != "" && sort.SearchStrings(option.compressibleContentTypes, contentType) >= 0 {
			// compressible Content-Type
			compressed := map[string][]byte{
				"gzip": func() []byte {
					buf := bytes.NewBuffer(nil)
					gzipWriter, _ := gzip.NewWriterLevel(buf, gzip.BestCompression)
					io.Copy(gzipWriter, bytes.NewReader(body))
					gzipWriter.Flush()
					gzipWriter.Close()
					return buf.Bytes()
				}(),
				"deflate": func() []byte {
					buf := bytes.NewBuffer(nil)
					deflateWriter, _ := zlib.NewWriterLevel(buf, zlib.BestCompression)
					io.Copy(deflateWriter, bytes.NewReader(body))
					deflateWriter.Flush()
					deflateWriter.Close()
					return buf.Bytes()
				}(),
			}

			handler = func(w http.ResponseWriter, r *http.Request) {
				algorithm := ""
				if r.Header.Get("Range") == "" {
					for _, acceptedEncoding := range ParseAcceptEncoding(r.Header.Values("Accept-Encoding")...) {
						if _, ok := compressed[acceptedEncoding.Algorithm]; ok {
							algorithm = acceptedEncoding.Algorithm
							break
						}
					}
				}

				if algorithm != "" {
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
					w.Header().Set("Content-Encoding", algorithm)
					w.Header().Set("Content-Type", contentType)
					w.WriteHeader(http.StatusOK)
					if r.Method != http.MethodHead {
						body := compressed[algorithm]
						io.CopyN(w, bytes.NewReader(body), int64(len(body)))
					}
				} else {
					w.Header().Set("Content-Type", contentType)
					http.ServeContent(w, r, name, modtime, bytes.NewReader(body))
				}
			}
		} else {
			handler = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				http.ServeContent(w, r, name, modtime, bytes.NewReader(body))
			}
		}

		if dir, filename := path.Split(upath); filename == "index.html" {
			if dir != "/" && strings.HasSuffix(dir, "/") {
				dir = strings.TrimSuffix(dir, "/")
			}
			contents[dir] = handler
			contents[upath] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				newPath := "./"
				if q := r.URL.RawQuery; q != "" {
					newPath += "?" + q
				}
				w.Header().Set("Location", newPath)
				w.WriteHeader(http.StatusMovedPermanently)
			})
		} else {
			contents[upath] = handler
		}

		return nil
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		upath = path.Clean(upath)

		next, ok := contents[upath]
		if !ok {
			option.notFound.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Accept-Encoding")
		next.ServeHTTP(w, r)
	})
}
