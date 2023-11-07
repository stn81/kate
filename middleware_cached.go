package kate

import (
	"bytes"
	"context"
	"fmt"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stn81/kate/log"
	"net/http"
)

const CacheSize = 1024

type responseInfo struct {
	Header http.Header
	Body   []byte
}

var responseCache *lru.Cache[string, *responseInfo]

func init() {
	var err error
	if responseCache, err = lru.New[string, *responseInfo](CacheSize); err != nil {
		panic(fmt.Errorf("failed to init response cache: %w", err))
	}
}

func getCacheKey(r *Request) string {
	bufferSize := len(r.Method) + len(r.RequestURI) + len(r.RawBody) + 2
	buf := bytes.NewBuffer(make([]byte, 0, bufferSize))
	buf.WriteString(r.Method)
	buf.WriteByte('|')
	buf.WriteString(r.RequestURI)
	buf.WriteByte('|')
	buf.Write(r.RawBody)
	return buf.String()
}

// Cached implements the cached middleware
func Cached(h ContextHandler) ContextHandler {
	f := func(ctx context.Context, w ResponseWriter, r *Request) {
		var (
			logger   = log.GetLogger(ctx)
			cacheKey = getCacheKey(r)
		)

		response, ok := responseCache.Get(cacheKey)
		if ok {
			logger.Info("use cached response")

			w.WriteHeader(http.StatusOK)
			for key, values := range response.Header {
				for _, v := range values {
					w.Header().Add(key, v)
				}
			}
			w.Write(response.Body)
			return
		}

		h.ServeHTTP(ctx, w, r)

		if w.StatusCode() == http.StatusOK {
			responseCache.Add(cacheKey, &responseInfo{
				Header: w.Header().Clone(),
				Body:   w.RawBody(),
			})
		}
	}
	return ContextHandlerFunc(f)
}
