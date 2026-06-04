package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id-123"

	ctx = WithRequestID(ctx, requestID)

	retrievedID := RequestIDFromContext(ctx)
	if retrievedID != requestID {
		t.Errorf("expected request ID %s, got %s", requestID, retrievedID)
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	id := RequestIDFromContext(ctx)
	if id != "" {
		t.Errorf("expected empty string for missing request ID, got %s", id)
	}
}

func TestRequestIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, 12345)

	id := RequestIDFromContext(ctx)
	if id != "" {
		t.Errorf("expected empty string for wrong type, got %s", id)
	}
}

func TestLoggerWithContext(t *testing.T) {
	logger := NewLogger()
	ctx := context.Background()

	// Without request ID
	loggerNoID := LoggerWithContext(logger, ctx)
	if loggerNoID == nil {
		t.Error("expected non-nil logger")
	}

	// With request ID
	ctx = WithRequestID(ctx, "test-id")
	loggerWithID := LoggerWithContext(logger, ctx)
	if loggerWithID == nil {
		t.Error("expected non-nil logger with request ID")
	}
}

func TestRequestIDMiddleware_ExistingID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := RequestIDFromContext(r.Context())
		if requestID != "existing-id" {
			t.Errorf("expected request ID existing-id, got %s", requestID)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "existing-id" {
		t.Errorf("expected X-Request-ID header existing-id, got %s", rec.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDMiddleware_GenerateID(t *testing.T) {
	var capturedID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected generated request ID, got empty string")
	}

	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Errorf("expected X-Request-ID header %s, got %s", capturedID, rec.Header().Get("X-Request-ID"))
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger := NewLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	})

	middleware := LoggingMiddleware(logger)(handler)

	req := httptest.NewRequest("POST", "/api/v1/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	ctx := WithRequestID(req.Context(), "log-test-id")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
}

func TestLoggingMiddleware_WithoutRequestID(t *testing.T) {
	logger := NewLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoggingMiddleware(logger)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code 404, got %d", rw.statusCode)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected response code 404, got %d", rec.Code)
	}
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write body without explicit WriteHeader
	rw.Write([]byte("test"))

	// Status should remain 200 (default)
	if rw.statusCode != http.StatusOK {
		t.Errorf("expected default status code 200, got %d", rw.statusCode)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}

	if id2 == "" {
		t.Error("expected non-empty request ID")
	}

	// IDs should be unique
	if id1 == id2 {
		t.Error("expected unique request IDs")
	}

	// Should be hex string (32 chars for 16 bytes)
	if len(id1) != 32 {
		t.Errorf("expected request ID length 32, got %d", len(id1))
	}

	// Should only contain hex characters
	for _, c := range id1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("request ID contains non-hex character: %c", c)
		}
	}
}

func TestLoggingMiddleware_LogsAllFields(t *testing.T) {
	logger := NewLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
	})

	middleware := LoggingMiddleware(logger)(handler)

	req := httptest.NewRequest("DELETE", "/api/v1/skills/test/my-skill/versions/1.0.0", nil)
	req.RemoteAddr = "192.168.1.1:54321"
	ctx := WithRequestID(req.Context(), "full-log-test")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", rec.Code)
	}
}

func BenchmarkRequestIDMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware(handler)
	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}
}

func BenchmarkLoggingMiddleware(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoggingMiddleware(logger)(handler)
	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}
}
