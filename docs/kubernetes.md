# Running Ollanta on Kubernetes

This guide covers deploying the **ollantaweb** server stack on Kubernetes. The architecture is designed for cloud-native operation: stateless app pods, externalized state, and independent scaling of every component.

---

## Design Principles

| Principle | How Ollanta implements it |
|-----------|--------------------------|
| **Resilient** | Graceful shutdown (SIGTERM + 15 s drain), liveness and readiness probes, degraded mode when search is down |
| **Customizable** | 100 % env-var driven config, pluggable search backend (`zincsearch` / `postgres`), pluggable index coordinator (`memory` / `pgnotify`) |
| **Atomic** | Distroless nonroot image, static Go binary, no shell, minimal attack surface |
| **Scalable** | Stateless app pods behind a Service, ZincSearch scaled independently, `pgnotify` coordinator for multi-replica indexing |
| **Ephemeral** | Zero local volumes on app pods, auto-migration on boot, indexes rebuilt from PostgreSQL on demand |

---

## Component Topology

```
                    ┌────────────────────┐
                    │   Ingress / LB     │
                    └─────────┬──────────┘
                              │
                   ┌──────────▼──────────┐
                   │   ollantaweb (HPA)  │   Deployment  ×N
                   │   stateless pods    │   port 8080
                   └───┬────────────┬────┘
                       │            │
            ┌──────────▼──┐  ┌─────▼───────────┐
            │  PostgreSQL  │  │   ZincSearch     │
            │  StatefulSet │  │   Deployment     │
            │  + PVC       │  │   + PVC          │
            └──────────────┘  └─────────────────┘
```

Each component is a **separate workload** with its own scaling policy. Unlike SonarQube (where Elasticsearch is embedded in the Compute Engine process), ZincSearch is fully external — connected over HTTP, addressed by a Kubernetes Service name.

---

## Prerequisites

- Kubernetes 1.26+
- `kubectl` configured for your cluster
- A container registry to push the `ollantaweb` image
- PostgreSQL 15+ (managed service or StatefulSet)
- ZincSearch 0.4.10+ (Deployment with PVC, or any Elasticsearch-compatible endpoint)

---

## Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ollanta
```

---

## Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ollanta-secrets
  namespace: ollanta
type: Opaque
stringData:
  database-url: "postgres://ollanta:CHANGE_ME@postgres.ollanta.svc:5432/ollanta?sslmode=require"
  zincsearch-password: "CHANGE_ME"
  jwt-secret: "CHANGE_ME_TO_A_RANDOM_64_CHAR_HEX"
```

> **Important**: `OLLANTA_JWT_SECRET` must be set explicitly in production. Without it, each pod generates a random secret at startup and tokens won't work across replicas.

---

## ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ollanta-config
  namespace: ollanta
data:
  OLLANTA_ADDR: ":8080"
  OLLANTA_ZINCSEARCH_URL: "http://zincsearch.ollanta.svc:4080"
  OLLANTA_ZINCSEARCH_USER: "admin"
  OLLANTA_SEARCH_BACKEND: "zincsearch"
  OLLANTA_INDEX_COORDINATOR: "pgnotify"
  OLLANTA_LOG_LEVEL: "info"
```

### Environment Variable Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLANTA_ADDR` | `:8080` | Listen address |
| `OLLANTA_DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `OLLANTA_ZINCSEARCH_URL` | `http://localhost:4080` | ZincSearch base URL |
| `OLLANTA_ZINCSEARCH_USER` | `admin` | ZincSearch Basic Auth user |
| `OLLANTA_ZINCSEARCH_PASSWORD` | `admin` | ZincSearch Basic Auth password |
| `OLLANTA_SEARCH_BACKEND` | `zincsearch` | `zincsearch` or `postgres` (fallback with no external dependency) |
| `OLLANTA_INDEX_COORDINATOR` | `memory` | `memory` (single pod) or `pgnotify` (multi-replica via LISTEN/NOTIFY) |
| `OLLANTA_JWT_SECRET` | *(random)* | HMAC-SHA256 signing key — **must be set for multi-replica** |
| `OLLANTA_JWT_EXPIRY` | `15m` | Access token lifetime |
| `OLLANTA_REFRESH_EXPIRY` | `720h` | Refresh token lifetime (30 days) |
| `OLLANTA_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

---

## Deployment: ollantaweb

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollantaweb
  namespace: ollanta
  labels:
    app.kubernetes.io/name: ollantaweb
    app.kubernetes.io/component: api
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: ollantaweb
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ollantaweb
        app.kubernetes.io/component: api
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: ollantaweb
          image: YOUR_REGISTRY/ollantaweb:latest
          ports:
            - containerPort: 8080
              name: http
              protocol: TCP
          envFrom:
            - configMapRef:
                name: ollanta-config
          env:
            - name: OLLANTA_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: database-url
            - name: OLLANTA_ZINCSEARCH_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: zincsearch-password
            - name: OLLANTA_JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: jwt-secret
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          startupProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 2
            periodSeconds: 3
            failureThreshold: 20
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: "1"
              memory: 512Mi
          securityContext:
            runAsNonRoot: true
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
```

### Key Details

- **`terminationGracePeriodSeconds: 30`** — gives the app 15 s for its internal graceful shutdown plus buffer for kubelet signaling.
- **Startup probe** — allows up to 60 s (`3 s × 20`) for initial migration + index configuration on cold start.
- **`readOnlyRootFilesystem: true`** — the distroless image writes nothing to disk; the binary is stateless.
- **`runAsNonRoot: true`** — enforced at pod level; the image already runs as nonroot UID.

---

## Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: ollantaweb
  namespace: ollanta
spec:
  selector:
    app.kubernetes.io/name: ollantaweb
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
```

---

## Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ollantaweb
  namespace: ollanta
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  ingressClassName: nginx
  rules:
    - host: ollanta.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ollantaweb
                port:
                  number: 8080
  tls:
    - hosts:
        - ollanta.example.com
      secretName: ollanta-tls
```

---

## HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ollantaweb
  namespace: ollanta
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ollantaweb
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

---

## PodDisruptionBudget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ollantaweb
  namespace: ollanta
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: ollantaweb
```

---

## ZincSearch Deployment

ZincSearch runs as a separate Deployment with its own PVC. It scales independently of ollantaweb.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zincsearch
  namespace: ollanta
  labels:
    app.kubernetes.io/name: zincsearch
    app.kubernetes.io/component: search
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: zincsearch
  template:
    metadata:
      labels:
        app.kubernetes.io/name: zincsearch
        app.kubernetes.io/component: search
    spec:
      containers:
        - name: zincsearch
          image: public.ecr.aws/zinclabs/zincsearch:0.4.10
          ports:
            - containerPort: 4080
              name: http
          env:
            - name: ZINC_FIRST_ADMIN_USER
              value: "admin"
            - name: ZINC_FIRST_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: zincsearch-password
            - name: ZINC_DATA_PATH
              value: /data
            - name: GIN_MODE
              value: release
          volumeMounts:
            - name: data
              mountPath: /data
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: "2"
              memory: 2Gi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: zincsearch-data
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: zincsearch-data
  namespace: ollanta
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: zincsearch
  namespace: ollanta
spec:
  selector:
    app.kubernetes.io/name: zincsearch
  ports:
    - port: 4080
      targetPort: http
      name: http
```

> **Note**: The ZincSearch image is scratch-based (no shell). Kubernetes probes should use `httpGet` on `/healthz` if your cluster version supports it, or `tcpSocket` on port 4080 as a fallback.

---

## Multi-Replica Indexing with pgnotify

When running multiple ollantaweb pods, set `OLLANTA_INDEX_COORDINATOR=pgnotify`. This uses PostgreSQL `LISTEN/NOTIFY` to coordinate index jobs:

1. When a scan is ingested, the receiving pod writes the job to a Postgres table and sends a `NOTIFY`.
2. Exactly one pod picks up the job via `LISTEN`, indexes the data in ZincSearch, and marks the job done.
3. No external queue (Redis, RabbitMQ) is needed — Postgres is the single coordination point.

```
Pod A (receives scan) ──NOTIFY──▶ PostgreSQL ──LISTEN──▶ Pod B (indexes)
```

With the default `memory` coordinator, each pod indexes independently — this is fine for single-replica deployments but can produce duplicate work with multiple pods.

---

## Fallback: No ZincSearch

If you want to eliminate the ZincSearch dependency entirely, set:

```yaml
OLLANTA_SEARCH_BACKEND: "postgres"
```

This uses PostgreSQL full-text search (`tsvector`/`tsquery`) for the `/api/v1/search` endpoint. Performance is good for moderate datasets and removes one moving part from the cluster. You can switch back to `zincsearch` at any time — the search index is rebuilt from PostgreSQL (source of truth) via `POST /admin/reindex`.

---

## Checklist

| Item | Status |
|------|--------|
| Namespace created | |
| Secrets populated (database-url, zincsearch-password, jwt-secret) | |
| `OLLANTA_JWT_SECRET` set (required for multi-replica token validation) | |
| `OLLANTA_INDEX_COORDINATOR=pgnotify` for multi-replica | |
| PostgreSQL accessible from cluster | |
| ZincSearch Deployment + PVC + Service running | |
| ollantaweb Deployment with probes, resources, security context | |
| Service + Ingress with TLS | |
| HPA configured | |
| PodDisruptionBudget set | |
| Network policies (optional — restrict ollantaweb → postgres, ollantaweb → zincsearch) | |
