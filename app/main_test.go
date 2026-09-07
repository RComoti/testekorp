package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestProjetoKorpHandler_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/projeto-korp", nil)
	rec := httptest.NewRecorder()

	handler := metricsMiddleware("/projeto-korp", projetoKorpHandler)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava código %d, mas obteve %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Esperava Content-Type application/json, obteve %s", contentType)
	}

	var payload ResponsePayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Erro ao decodificar JSON de resposta: %v", err)
	}

	if payload.Nome != "Projeto Korp" {
		t.Errorf("Esperava nome 'Projeto Korp', obteve '%s'", payload.Nome)
	}

	parsedTime, err := time.Parse(time.RFC3339, payload.Horario)
	if err != nil {
		t.Fatalf("Horário retornado não está no formato RFC3339: %v", err)
	}

	if time.Since(parsedTime) > 5*time.Second {
		t.Errorf("Horário gerado está muito distante do horário atual: %v", parsedTime)
	}
}

func TestProjetoKorpHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/projeto-korp", nil)
	rec := httptest.NewRecorder()

	handler := metricsMiddleware("/projeto-korp", projetoKorpHandler)
	handler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Esperava status %d para método POST, mas obteve %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler := metricsMiddleware("/healthz", healthHandler)
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status %d, obteve %d", http.StatusOK, rec.Code)
	}

	var payload HealthPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	if payload.Status != "UP" {
		t.Errorf("Esperava status UP, obteve %s", payload.Status)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	serviceAvailability.Set(1)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler := promhttp.Handler()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Esperava status %d em /metrics, obteve %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "service_availability") {
		t.Errorf("Esperava encontrar métrica 'service_availability' na resposta de /metrics")
	}
	if !strings.Contains(body, "http_requests_total") {
		t.Errorf("Esperava encontrar métrica 'http_requests_total' na resposta de /metrics")
	}
}
