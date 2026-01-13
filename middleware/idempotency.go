package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"genimage/auth"
	"genimage/model"

	"gorm.io/gorm"
)

const (
	IdempotencyHeader  = "Idempotency-Key"
	idempotencyTTL     = 24 * time.Hour
	statusPending      = "pending"
	statusCompleted    = "completed"
	maxRequestBodySize = 10 * 1024 * 1024 // 10MB limit for request body
)

var headerBlacklist = map[string]bool{
	"Set-Cookie":        true,
	"Date":              true,
	"Server":            true,
	"Transfer-Encoding": true,
	"Connection":        true,
	"Content-Length":    true,
}

type IdempotencyConfig struct {
	EnabledPaths []string
}

func Idempotency(db *gorm.DB, config IdempotencyConfig) func(http.Handler) http.Handler {
	pathSet := make(map[string]bool)
	for _, p := range config.EnabledPaths {
		pathSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !pathSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(IdempotencyHeader)
			user := auth.GetUserFromContext(r.Context())

			if key == "" || user == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "request body too large or unreadable", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			bodyHash := hashBytes(bodyBytes)

			var existing model.IdempotencyKey
			err = db.Where("user_id = ? AND key = ?", user.ID, key).First(&existing).Error

			if err == nil {
				// Check expiration FIRST (fix zombie lock issue)
				if time.Now().After(existing.ExpiresAt) {
					db.Delete(&existing)
					// Fall through to create new record
				} else {
					// Validate method and path match
					if existing.Method != r.Method || existing.Path != r.URL.Path {
						http.Error(w, "idempotency key mismatch", http.StatusUnprocessableEntity)
						return
					}

					// Validate body hash match (fix: was not being checked)
					if existing.BodyHash != bodyHash {
						http.Error(w, "idempotency key mismatch", http.StatusUnprocessableEntity)
						return
					}

					if existing.Status == statusPending {
						http.Error(w, "request in progress", http.StatusConflict)
						return
					}

					if existing.Status == statusCompleted {
						writeCachedResponse(w, &existing)
						return
					}
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				// DB error (not "record not found") - return 500 instead of masking
				log.Printf("idempotency: database error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			record := &model.IdempotencyKey{
				UserID:    user.ID,
				Key:       key,
				Method:    r.Method,
				Path:      r.URL.Path,
				BodyHash:  bodyHash,
				Status:    statusPending,
				ExpiresAt: time.Now().Add(idempotencyTTL),
			}

			if err := db.Create(record).Error; err != nil {
				// Any create error should block execution to maintain idempotency guarantee
				http.Error(w, "request in progress", http.StatusConflict)
				return
			}

			rec := NewResponseRecorder(w)

			// Handle panic: delete pending record to prevent zombie locks
			defer func() {
				if p := recover(); p != nil {
					db.Delete(record)
					panic(p) // re-panic after cleanup
				}
			}()

			next.ServeHTTP(rec, r)

			// Only cache successful, non-truncated responses
			if rec.Code() >= 200 && rec.Code() < 300 && !rec.IsTruncated() {
				headersJSON, err := json.Marshal(filterHeaders(rec.Headers()))
				if err != nil {
					log.Printf("idempotency: failed to marshal headers: %v", err)
				}
				if err := db.Model(record).Updates(map[string]interface{}{
					"status":           statusCompleted,
					"response_status":  rec.Code(),
					"response_headers": string(headersJSON),
					"response_body":    rec.Body(),
				}).Error; err != nil {
					log.Printf("idempotency: failed to update record: %v", err)
					// Delete the pending record so retry can work
					db.Delete(record)
				}
			} else {
				db.Delete(record)
			}
		})
	}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func filterHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		// Use canonical key for case-insensitive blacklist check
		if !headerBlacklist[http.CanonicalHeaderKey(k)] && len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func writeCachedResponse(w http.ResponseWriter, record *model.IdempotencyKey) {
	if record.ResponseHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(record.ResponseHeaders), &headers); err == nil {
			for k, v := range headers {
				w.Header().Set(k, v)
			}
		}
	}
	w.Header().Set("X-Idempotency-Replayed", "true")
	w.WriteHeader(record.ResponseStatus)
	w.Write(record.ResponseBody)
}

func CleanExpiredIdempotencyKeys(db *gorm.DB) error {
	return db.Where("expires_at < ?", time.Now()).Delete(&model.IdempotencyKey{}).Error
}
