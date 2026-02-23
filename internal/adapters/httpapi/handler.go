package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/usecase"
	"github.com/go-chi/chi/v5"
)

type ctxKey string

const (
	timeFormat             = "2006-01-02T15:04:05.999999999Z07:00"
	tenantIDCtxKey  ctxKey = "tenant_id"
	apiActorCtxKey  ctxKey = "api_actor"
	maxJSONBodySize        = 1 << 20
)

type Handler struct {
	kvService     *usecase.KVService
	recordService *usecase.RecordService
	authService   *usecase.AuthService
	auditService  *usecase.AuditService
	schemaService *usecase.SchemaService
	readinessFn   func(context.Context) error
	extraMetrics  func() map[string]int64
	metrics       handlerMetrics
}

type Option func(*Handler)

func WithReadinessCheck(fn func(context.Context) error) Option {
	return func(h *Handler) {
		h.readinessFn = fn
	}
}

func WithExtraMetrics(fn func() map[string]int64) Option {
	return func(h *Handler) {
		h.extraMetrics = fn
	}
}

func NewHandler(kvService *usecase.KVService, recordService *usecase.RecordService, authService *usecase.AuthService, auditService *usecase.AuditService, schemaService *usecase.SchemaService, opts ...Option) *Handler {
	h := &Handler{kvService: kvService, recordService: recordService, authService: authService, auditService: auditService, schemaService: schemaService}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", h.healthz)
	r.Get("/readyz", h.readyz)
	r.Get("/metricsz", h.metricsz)
	r.Get("/openapi.json", h.openapi)

	r.Group(func(pr chi.Router) {
		pr.Use(h.requireAPIKey)
		pr.Use(h.requestLogger)
		pr.Get("/v1/kv", h.scan)
		pr.Put("/v1/kv/{key}", h.upsert)
		pr.Get("/v1/kv/{key}", h.get)
		pr.Delete("/v1/kv/{key}", h.delete)

		pr.Get("/v1/collections/{collection}/records", h.listRecords)
		pr.Put("/v1/collections/{collection}/records/{id}", h.upsertRecord)
		pr.Get("/v1/collections/{collection}/records/{id}", h.getRecord)
		pr.Delete("/v1/collections/{collection}/records/{id}", h.deleteRecord)
		pr.Post("/v1/collections/{collection}/records:bulk-upsert", h.bulkUpsertRecords)
		pr.Post("/v1/collections/{collection}/records:bulk-delete", h.bulkDeleteRecords)
		pr.Put("/v1/collections/{collection}/schema", h.upsertCollectionSchema)
		pr.Get("/v1/collections/{collection}/schema", h.getCollectionSchema)
		pr.Delete("/v1/collections/{collection}/schema", h.deleteCollectionSchema)
		pr.Get("/v1/audit/events", h.listAuditEvents)
	})

	return r
}

type upsertRequest struct {
	Category string          `json:"category"`
	Value    json.RawMessage `json:"value"`
}

type itemResponse struct {
	Key       string          `json:"key"`
	Category  string          `json:"category"`
	Value     json.RawMessage `json:"value"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type recordResponse struct {
	ID         string          `json:"id"`
	Collection string          `json:"collection"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

type bulkUpsertRequest struct {
	Items []usecase.BulkUpsertItem `json:"items"`
}

type bulkDeleteRequest struct {
	IDs []string `json:"ids"`
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)

	var req upsertRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	item, err := h.kvService.Upsert(r.Context(), domain.Item{
		Key:      key,
		Category: req.Category,
		Value:    req.Value,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toItemResponse(item))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	item, err := h.kvService.Get(r.Context(), key)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toItemResponse(item))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	deleted, err := h.kvService.Delete(r.Context(), key)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": deleted})
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}

	items, err := h.kvService.Scan(r.Context(), domain.ScanFilter{
		Category: r.URL.Query().Get("category"),
		Prefix:   r.URL.Query().Get("prefix"),
		AfterKey: r.URL.Query().Get("after"),
		Limit:    limit,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}

	result := make([]itemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toItemResponse(item))
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) upsertRecord(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")
	tenantID := tenantIDFromContext(r.Context())
	actor := actorFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	var data json.RawMessage
	if err := decoder.Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	rec, err := h.recordService.Upsert(r.Context(), domain.Record{
		TenantID:   tenantID,
		Collection: collection,
		ID:         id,
		Data:       data,
	}, mutationMetaFromRequest(r, actor))
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRecordResponse(rec))
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")
	tenantID := tenantIDFromContext(r.Context())

	rec, err := h.recordService.Get(r.Context(), tenantID, collection, id)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRecordResponse(rec))
}

func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")
	tenantID := tenantIDFromContext(r.Context())
	actor := actorFromContext(r.Context())

	deleted, err := h.recordService.Delete(r.Context(), tenantID, collection, id, mutationMetaFromRequest(r, actor))
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": deleted})
}

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())

	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}

	records, err := h.recordService.List(
		r.Context(),
		tenantID,
		collection,
		domain.RecordListFilter{
			Prefix: r.URL.Query().Get("prefix"),
			After:  r.URL.Query().Get("after"),
			Limit:  limit,
			JSON: domain.JSONPathFilter{
				Path:  strings.TrimSpace(r.URL.Query().Get("json_path")),
				Op:    strings.TrimSpace(r.URL.Query().Get("json_op")),
				Value: r.URL.Query().Get("json_value"),
			},
		},
	)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	result := make([]recordResponse, 0, len(records))
	for _, rec := range records {
		result = append(result, toRecordResponse(rec))
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) bulkUpsertRecords(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())
	actor := actorFromContext(r.Context())

	if cached, ok := h.readIdempotentResponse(r.Context(), tenantID, collection, "bulk-upsert", r.Header.Get("Idempotency-Key")); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req bulkUpsertRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	records, err := h.recordService.BulkUpsert(r.Context(), tenantID, collection, req.Items, mutationMetaFromRequest(r, actor))
	if err != nil {
		handleDomainError(w, err)
		return
	}

	result := make([]recordResponse, 0, len(records))
	for _, rec := range records {
		result = append(result, toRecordResponse(rec))
	}
	payload := map[string]any{"items": result}
	h.writeIdempotentResponse(r.Context(), tenantID, collection, "bulk-upsert", r.Header.Get("Idempotency-Key"), payload)
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) bulkDeleteRecords(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())
	actor := actorFromContext(r.Context())

	if cached, ok := h.readIdempotentResponse(r.Context(), tenantID, collection, "bulk-delete", r.Header.Get("Idempotency-Key")); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req bulkDeleteRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	count, err := h.recordService.BulkDelete(r.Context(), tenantID, collection, req.IDs, mutationMetaFromRequest(r, actor))
	if err != nil {
		handleDomainError(w, err)
		return
	}

	payload := map[string]any{"deleted": count}
	h.writeIdempotentResponse(r.Context(), tenantID, collection, "bulk-delete", r.Header.Get("Idempotency-Key"), payload)
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) upsertCollectionSchema(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	var schemaJSON json.RawMessage
	if err := decoder.Decode(&schemaJSON); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	cs, err := h.schemaService.Upsert(r.Context(), tenantID, collection, schemaJSON)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSchemaResponse(cs))
}

func (h *Handler) getCollectionSchema(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())

	cs, err := h.schemaService.Get(r.Context(), tenantID, collection)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSchemaResponse(cs))
}

func (h *Handler) deleteCollectionSchema(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	tenantID := tenantIDFromContext(r.Context())

	deleted, err := h.schemaService.Delete(r.Context(), tenantID, collection)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": deleted})
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	if h.readinessFn != nil {
		if err := h.readinessFn(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not ready")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) metricsz(w http.ResponseWriter, _ *http.Request) {
	body := map[string]int64{
		"http_requests_total":           h.metrics.httpRequestsTotal.Load(),
		"http_request_latency_ms_total": h.metrics.httpRequestLatencyMsTotal.Load(),
		"record_write_total":            h.metrics.recordWriteTotal.Load(),
	}
	if h.extraMetrics != nil {
		for k, v := range h.extraMetrics() {
			body[k] = v
		}
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) openapi(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, openapiSpec())
}

func (h *Handler) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}

	afterID := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "after_id must be integer")
			return
		}
		afterID = v
	}

	events, err := h.auditService.List(r.Context(), domain.AuditFilter{
		TenantID:      tenantID,
		AggregateType: r.URL.Query().Get("aggregate_type"),
		AggregateID:   r.URL.Query().Get("aggregate_id"),
		Action:        r.URL.Query().Get("action"),
		AfterID:       afterID,
		Limit:         limit,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (h *Handler) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if token == "" {
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				token = strings.TrimSpace(auth[7:])
			}
		}

		apiKey, err := h.authService.Authenticate(r.Context(), token)
		if err != nil {
			if errors.Is(err, usecase.ErrUnauthorized) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		ctx := context.WithValue(r.Context(), tenantIDCtxKey, apiKey.TenantID)
		ctx = context.WithValue(ctx, apiActorCtxKey, apiKey.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type schemaResponse struct {
	Collection string          `json:"collection"`
	Schema     json.RawMessage `json:"schema"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

func toSchemaResponse(cs domain.CollectionSchema) schemaResponse {
	return schemaResponse{
		Collection: cs.Collection,
		Schema:     cs.Schema,
		CreatedAt:  cs.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:  cs.UpdatedAt.UTC().Format(timeFormat),
	}
}

func toItemResponse(item domain.Item) itemResponse {
	return itemResponse{
		Key:       item.Key,
		Category:  item.Category,
		Value:     item.Value,
		CreatedAt: item.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt: item.UpdatedAt.UTC().Format(timeFormat),
	}
}

func toRecordResponse(rec domain.Record) recordResponse {
	return recordResponse{
		ID:         rec.ID,
		Collection: rec.Collection,
		Data:       rec.Data,
		CreatedAt:  rec.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:  rec.UpdatedAt.UTC().Format(timeFormat),
	}
}

func parseLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be integer")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		log.Printf("encode json response: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func handleDomainError(w http.ResponseWriter, err error) {
	var schemaErr *domain.ErrSchemaViolation
	switch {
	case errors.As(err, &schemaErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  "schema validation failed",
			"errors": schemaErr.Errors,
		})
	case errors.Is(err, domain.ErrInvalidKey), errors.Is(err, domain.ErrInvalidCategory):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrInvalidFilter):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func ensureEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return errors.New("extra json tokens")
}

func tenantIDFromContext(ctx context.Context) string {
	tenant, _ := ctx.Value(tenantIDCtxKey).(string)
	return tenant
}

func actorFromContext(ctx context.Context) string {
	actor, _ := ctx.Value(apiActorCtxKey).(string)
	if actor == "" {
		return "api"
	}
	return actor
}

func mutationMetaFromRequest(r *http.Request, actor string) domain.MutationMetadata {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
	}
	correlationID := strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
	if correlationID == "" {
		correlationID = strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
	}
	causationID := strings.TrimSpace(r.Header.Get("X-Causation-Id"))
	if causationID == "" {
		causationID = strings.TrimSpace(r.Header.Get("X-Causation-ID"))
	}

	return domain.MutationMetadata{
		Actor:          actor,
		Source:         "api",
		RequestID:      requestID,
		CorrelationID:  correlationID,
		CausationID:    causationID,
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		OccurredAt:     time.Now().UTC(),
	}
}

type handlerMetrics struct {
	httpRequestsTotal         atomic.Int64
	httpRequestLatencyMsTotal atomic.Int64
	recordWriteTotal          atomic.Int64
}

func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		durationMs := time.Since(start).Milliseconds()
		route := routePattern(r)
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
		}
		tenantID := tenantIDFromContext(r.Context())

		h.metrics.httpRequestsTotal.Add(1)
		h.metrics.httpRequestLatencyMsTotal.Add(durationMs)
		if isWriteRoute(r.Method, route) {
			h.metrics.recordWriteTotal.Add(1)
		}

		log.Printf("request method=%s route=%s status=%d duration_ms=%d request_id=%s tenant_id=%s", r.Method, route, ww.status, durationMs, requestID, tenantID)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func routePattern(r *http.Request) string {
	if ctx := chi.RouteContext(r.Context()); ctx != nil {
		if p := strings.TrimSpace(ctx.RoutePattern()); p != "" {
			return p
		}
	}
	return r.URL.Path
}

func isWriteRoute(method, route string) bool {
	switch method {
	case http.MethodPut, http.MethodPost, http.MethodDelete:
		return strings.Contains(route, "/records") || strings.HasPrefix(route, "/v1/kv/")
	default:
		return false
	}
}

func idempotencyKey(tenantID, collection, op, token string) string {
	return "idempotency/" + tenantID + "/" + collection + "/" + op + "/" + token
}

func (h *Handler) readIdempotentResponse(ctx context.Context, tenantID, collection, op, token string) (map[string]any, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, false
	}

	item, err := h.kvService.Get(ctx, idempotencyKey(tenantID, collection, op, token))
	if err != nil {
		return nil, false
	}

	var payload map[string]any
	if err := json.Unmarshal(item.Value, &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func (h *Handler) writeIdempotentResponse(ctx context.Context, tenantID, collection, op, token string, payload map[string]any) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}

	_, _ = h.kvService.Upsert(ctx, domain.Item{
		Key:      idempotencyKey(tenantID, collection, op, token),
		Category: "idempotency/" + tenantID,
		Value:    encoded,
	})
}

func openapiSpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "dbapi",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/v1/collections/{collection}/records": map[string]any{
				"get": map[string]any{"summary": "List records"},
			},
			"/v1/collections/{collection}/schema": map[string]any{
				"put":    map[string]any{"summary": "Upsert collection schema"},
				"get":    map[string]any{"summary": "Get collection schema"},
				"delete": map[string]any{"summary": "Delete collection schema"},
			},
			"/v1/collections/{collection}/records/{id}": map[string]any{
				"put":    map[string]any{"summary": "Upsert record"},
				"get":    map[string]any{"summary": "Get record"},
				"delete": map[string]any{"summary": "Delete record"},
			},
			"/v1/collections/{collection}/records:bulk-upsert": map[string]any{
				"post": map[string]any{"summary": "Bulk upsert records"},
			},
			"/v1/collections/{collection}/records:bulk-delete": map[string]any{
				"post": map[string]any{"summary": "Bulk delete records"},
			},
			"/v1/audit/events": map[string]any{
				"get": map[string]any{"summary": "List audit events"},
			},
		},
	}
}
