# Guia de Deploy de Microsserviços com CroMedia no Kubernetes

Este documento orienta sobre como realizar o deploy e orquestração de microsserviços baseados no CroMedia em clusters Kubernetes de produção utilizando imagens Docker minimalistas (`scratch`).

---

## 🐋 1. Imagem Docker Otimizada (Multi-Stage)

Para obter o menor tamanho de imagem possível e eliminar vetores de vulnerabilidade, o CroMedia utiliza imagens baseadas em `scratch` contendo apenas as dependências mínimas de TLS e o binário estaticamente compilado.

Abaixo está o `Dockerfile` recomendado para produção:

```dockerfile
# Estágio de Compilação
FROM golang:1.25.0-alpine AS builder
RUN apk add --no-cache git build-base ca-certificates

WORKDIR /app
COPY . .

# Compilação estática com linker otimizado e exclusão de CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -tags "legacy legacy_avi legacy_asf legacy_rm legacy_mp2 legacy_codecs" \
    -o cromedia main.go

# Estágio Final Ultraleve
FROM scratch

# Certificados TLS para requisições HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/cromedia /bin/cromedia

# Porta do microsserviço (se expuser API HTTP/RTMP/gRPC)
EXPOSE 8080

ENTRYPOINT ["/bin/cromedia"]
```

Com este Dockerfile, a imagem resultante terá cerca de **15 MB** a **25 MB**, otimizando significativamente a latência de pull e tempos de cold start.

---

## ☸️ 2. Manifesto de Deployment Kubernetes

Abaixo está um manifesto YAML completo contendo o `Deployment`, `Service`, `HPA` (Horizontal Pod Autoscaler) e politicas de recursos recomendadas para o CroMedia.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cromedia-transcoder
  namespace: media-processing
  labels:
    app: cromedia-transcoder
spec:
  replicas: 3
  selector:
    matchLabels:
      app: cromedia-transcoder
  template:
    metadata:
      labels:
        app: cromedia-transcoder
    spec:
      containers:
      - name: cromedia
        image: gcr.io/meu-projeto/cromedia:v0.8.0
        command: ["/bin/cromedia"]
        args: ["--strict"] # Bloqueia fallback local se necessário
        ports:
        - containerPort: 8080
          name: http
        resources:
          limits:
            cpu: "2"
            memory: 1Gi
          requests:
            cpu: "500m"
            memory: 256Mi
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 10001
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 2
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: cromedia-service
  namespace: media-processing
spec:
  selector:
    app: cromedia-transcoder
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  type: ClusterIP
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cromedia-autoscaler
  namespace: media-processing
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cromedia-transcoder
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 75
```

---

## ⚡ 3. Considerações de Performance e Recursos

1. **CPU Pinning**: O CroMedia realiza processamento altamente paralelo. Defina quotas de CPU compatíveis nos limites de recursos do Kubernetes para evitar throttling do scheduler do kernel.
2. **BufferPool**: Monitore a telemetria do `BufferPool` para calibrar o limite de memória do Pod. Caso o limite seja muito baixo, o Kubernetes enviará um sinal `OOMKilled`.
3. **Escalonamento**: Como o processamento de mídias é CPU-Bound, configure o Horizontal Pod Autoscaler (`HPA`) baseado na métrica de utilização média de CPU.
