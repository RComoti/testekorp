# ==========================================
# Estágio 1: Build da Aplicação
# ==========================================
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Instalar certificados raiz e dados de fuso horário
RUN apk --no-cache add ca-certificates tzdata

# Copiar todo o código-fonte
COPY go.mod ./
COPY . .

# Resolver e baixar dependências com base no código-fonte
RUN go mod tidy && go mod download

# Executar testes unitários durante o processo de build
RUN CGO_ENABLED=0 go test -v ./...

# Compilar binário estático e enxuto (-s -w remove símbolos para reduzir tamanho)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o http-server-projeto-korp .

# ==========================================
# Estágio 2: Imagem Final de Execução (Lightweight & Secure)
# ==========================================
FROM alpine:3.20

# Criar usuário não-root por segurança
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copiar certificados e timezone
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copiar o binário gerado
COPY --from=builder /app/http-server-projeto-korp .

# Ajustar permissões para o usuário não-root
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# Healthcheck interno do container
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["./http-server-projeto-korp"]
