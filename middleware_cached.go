package kate

import (
	"bytes"
	"context"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stn81/kate/log"
	"net/http"
	"time"
)

// Cached implements the cached middleware
func Cached(size int, ttl time.Duration) Middleware {
	return newCachedProxy(size, ttl)
}

type responseInfo struct {
	Header http.Header
	Body   []byte
}

type cachedProxy struct {
	cache *expirable.LRU[string, *responseInfo]
}

func newCachedProxy(size int, ttl time.Duration) *cachedProxy {
	return &cachedProxy{
		cache: expirable.NewLRU[string, *responseInfo](size, nil, ttl),
	}
}

func (p *cachedProxy) Proxy(h ContextHandler) ContextHandler {
	f := func(ctx context.Context, w ResponseWriter, r *Request) {
		var (
			logger   = log.GetLogger(ctx)
			cacheKey = p.getCacheKey(r)
		)

		response, ok := p.cache.Get(cacheKey)
		if ok {
			logger.Info("use cached response")

			w.WriteHeader(http.StatusOK)
			for key, values := range response.Header {
				for _, v := range values {
					w.Header().Add(key, v)
				}
			}
			_, _ = w.Write(response.Body)
			return
		}

		h.ServeHTTP(ctx, w, r)

		if w.StatusCode() == http.StatusOK {
			p.cache.Add(cacheKey, &responseInfo{
				Header: w.Header().Clone(),
				Body:   w.RawBody(),
			})
		}
	}
	return ContextHandlerFunc(f)
}

func (p *cachedProxy) getCacheKey(r *Request) string {
	bufferSize := len(r.Method) + len(r.RequestURI) + len(r.RawBody) + 2
	buf := bytes.NewBuffer(make([]byte, 0, bufferSize))
	buf.WriteString(r.Method)
	buf.WriteByte('|')
	buf.WriteString(r.RequestURI)
	buf.WriteByte('|')
	buf.Write(r.RawBody)
	return buf.String()
}
