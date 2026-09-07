package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Fuso horário do Brasil (America/Sao_Paulo, GMT-3 / UTC-3)
var brazilLocation *time.Location

func init() {
	var err error
	brazilLocation, err = time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Fallback para fuso horário fixo UTC-3 (Horário de Brasília / São Paulo)
		brazilLocation = time.FixedZone("America/Sao_Paulo", -3*60*60)
	}
}

// ResponsePayload representa a estrutura de resposta exigida pelo desafio
type ResponsePayload struct {
	Nome    string `json:"nome"`
	Horario string `json:"horario"`
}

// HealthPayload representa a resposta do endpoint de integridade
type HealthPayload struct {
	Status string `json:"status"`
}

// Métricas do Prometheus
var (
	// Métrica obrigatória 1: Disponibilidade do serviço (1 = UP, 0 = DOWN)
	serviceAvailability = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "service_availability",
		Help: "Disponibilidade do serviço http-server-projeto-korp (1 = UP, 0 = DOWN)",
	})

	// Métrica obrigatória 2: Volume total de requisições por método, endpoint e código HTTP
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisições HTTP recebidas pelo serviço",
	}, []string{"method", "endpoint", "status"})

	// Métrica adicional recomendada: Duração / latência das requisições em segundos
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duração das requisições HTTP em segundos",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})
)

// responseWriterInterceptor captura o status code para alimentar as métricas do Prometheus
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rwi *responseWriterInterceptor) WriteHeader(statusCode int) {
	rwi.statusCode = statusCode
	rwi.ResponseWriter.WriteHeader(statusCode)
}

// metricsMiddleware intercepta as requisições e registra métricas de volume, status e latência
func metricsMiddleware(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		interceptor := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}

		next(interceptor, r)

		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(interceptor.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, endpoint, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
	}
}

func projetoKorpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	response := ResponsePayload{
		Nome:    "Projeto Korp",
		Horario: time.Now().In(brazilLocation).Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERRO] Falha ao serializar resposta JSON: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthPayload{Status: "UP"})
}

func main() {
	// Define disponibilidade como 1 (UP)
	serviceAvailability.Set(1)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Endpoint principal do desafio protegido pelo middleware de métricas
	mux.HandleFunc("/projeto-korp", metricsMiddleware("/projeto-korp", projetoKorpHandler))

	// Endpoint de healthcheck
	mux.HandleFunc("/healthz", metricsMiddleware("/healthz", healthHandler))

	// Endpoint padrão do Prometheus para exportação de métricas
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// Captura de sinais para encerramento gracioso (Graceful Shutdown)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Servidor http-server-projeto-korp iniciado na porta %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Erro ao iniciar servidor HTTP: %v", err)
		}
	}()

	<-stopChan
	log.Println("[INFO] Encerrando servidor http-server-projeto-korp...")

	// Sinaliza indisponibilidade antes de encerrar
	serviceAvailability.Set(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("[ERRO] Erro ao encerrar servidor: %v", err)
	}

	log.Println("[INFO] Servidor finalizado com sucesso.")
}
