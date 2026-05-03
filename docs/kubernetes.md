# Running Ollanta on Kubernetes

This guide covers deploying the Ollanta server stack on Kubernetes. The architecture is designed for cloud-native operation: stateless web and compute pods, externalized state, and independent scaling of every component.

---

## Design Principles

| Principle | How Ollanta implements it |
|-----------|--------------------------|
| **Resilient** | Graceful shutdown (SIGTERM + 15 s drain), liveness and readiness probes, degraded mode when search is down |
| **Customizable** | 100 % env-var driven config, pluggable search backend (`zincsearch` / `postgres`) |
| **Atomic** | Distroless nonroot image, static Go binary, no shell, minimal attack surface |
| **Scalable** | Stateless web pods behind a Service, separate compute, index, and webhook worker deployments, ZincSearch scaled independently |
| **Ephemeral** | Zero local volumes on web and worker pods, explicit migration job for production, indexes rebuilt from PostgreSQL on demand |

---

## Component Topology

```
                    ┌────────────────────┐
                    │   Ingress / LB     │
                    └─────────┬──────────┘
                              │
                    ┌──────────▼──────────┐
                    │   ollantaweb (HPA)  │   Deployment ×N
                    │   web / query role  │   port 8080
                    └───┬────────────┬────┘
                        │            │
                        │            └──────────────────────────────┐
                        │                                           │
                    ┌────────▼─────┐                              ┌──────▼─────────┐
                    │ PostgreSQL    │                              │ ollantaworker  │
                    │ StatefulSet   │                              │ compute role   │
                    │ + PVC         │                              │ Deployment ×N  │
                    └──────┬────────┘                              └──────┬─────────┘
                      │                                              │
                      │                            emits durable jobs │
                      │                                              │
                      ┌─────────▼─────────┐                         ┌──────────▼──────────┐
                      │  ollantaindexer    │                         │ ollantawebhookworker│
                      │  projection role   │                         │ notification role   │
                      │  Deployment ×N     │                         │ Deployment ×N       │
                      └─────────┬──────────┘                         └─────────────────────┘
                      │
                    ┌──────▼───────┐
                    │  ZincSearch   │
                    │  Deployment   │
                    │  + PVC        │
                    └───────────────┘
```

              Each component is a separate workload with its own scaling policy. The web role accepts and serves API traffic, the compute role materializes scans and emits durable side-effect jobs, the indexer projects searchable state into ZincSearch, and the webhook worker handles external delivery retry loops. Unlike SonarQube (where Elasticsearch is embedded in the Compute Engine process), ZincSearch is fully external and addressed over HTTP.

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
  OLLANTA_LOG_LEVEL: "info"
  OLLANTA_AUTO_MIGRATE: "false"
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
| `OLLANTA_SCANNER_TOKEN` | *(empty)* | Shared token accepted for scanner pushes to `POST /api/v1/scans` |
| `OLLANTA_JWT_SECRET` | *(required unless dev opt-in)* | HMAC-SHA256 signing key; set a stable secret for every pod |
| `OLLANTA_ALLOW_RANDOM_JWT_SECRET` | `false` | Development-only opt-in for random JWT secrets |
| `OLLANTA_JWT_EXPIRY` | `15m` | Access token lifetime |
| `OLLANTA_REFRESH_EXPIRY` | `720h` | Refresh token lifetime (30 days) |
| `OLLANTA_AUTO_MIGRATE` | `true` locally | Set `false` on production API and worker roles after running the migration job |
| `OLLANTA_CORS_ALLOWED_ORIGINS` | same-origin/local dev | Comma-separated allowlist of browser origins |
| `OLLANTA_CORS_ALLOW_UNSAFE_WILDCARD` | `false` | Development-only opt-in for `*` CORS |
| `OLLANTA_HTTP_MAX_BODY_BYTES` | `52428800` | Maximum scan report request body size |
| `OLLANTA_POSTGRES_MAX_CONNS` | `25` | Maximum PostgreSQL pool size per pod |
| `OLLANTA_SCAN_QUEUE_MAX_ACCEPTED` | `0` | Optional accepted scan queue pressure limit; `0` disables |
| `OLLANTA_SCAN_QUEUE_MAX_RUNNING` | `0` | Optional running scan queue pressure limit; `0` disables |
| `OLLANTA_SCAN_QUEUE_MAX_OLDEST_ACCEPTED_AGE` | `0s` | Optional age-based scan intake pressure limit; `0s` disables |
| `OLLANTA_SCAN_QUEUE_RETRY_AFTER` | `30s` | Retry hint used for scan intake `429` responses |
| `OLLANTA_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Development Defaults To Avoid In Production

| Development convenience | Production alternative |
|-------------------------|------------------------|
| `OLLANTA_ALLOW_RANDOM_JWT_SECRET=true` | Set a stable high-entropy `OLLANTA_JWT_SECRET` in Kubernetes Secret storage. |
| Wildcard CORS with `OLLANTA_CORS_ALLOW_UNSAFE_WILDCARD=true` | Set `OLLANTA_CORS_ALLOWED_ORIGINS` to the exact UI origins. |
| `OLLANTA_AUTO_MIGRATE=true` on every pod | Run the `ollantamigrate` Job, then set `OLLANTA_AUTO_MIGRATE=false` on API and worker pods. |
| Local Compose observability defaults | Use managed or cluster observability with explicit retention, access control, and scrape targets. |

---

## Migration Job

For production, run migrations as a one-shot deploy job and set `OLLANTA_AUTO_MIGRATE=false` on long-running API and worker pods. The migrator uses the same image and acquires a PostgreSQL advisory lock, so repeated or concurrent deploy attempts are serialized by the database.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: ollantamigrate
  namespace: ollanta
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: ollantamigrate
          image: YOUR_REGISTRY/ollantaweb:latest
          command: ["/ollantamigrate"]
          envFrom:
            - configMapRef:
                name: ollanta-config
          env:
            - name: OLLANTA_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: database-url
            - name: OLLANTA_JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: ollanta-secrets
                  key: jwt-secret
```

After this job succeeds, API and worker pods verify that the latest embedded migration exists in `schema_migrations` before serving or processing work.

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

## Durable Worker Behavior on Multiple Replicas

`ollantaweb` no longer depends on a local in-process indexing queue. The pod that receives `POST /api/v1/scans` writes an accepted `scan_job` to PostgreSQL. Any `ollantaworker` replica can claim that job, materialize the scan, and enqueue durable `index_jobs` and `webhook_jobs`. Any `ollantaindexer` or `ollantawebhookworker` replica can then claim those jobs.

```
API Pod A ──scan_jobs──▶ PostgreSQL ◀──claims── Worker Pod N
Worker Pod N ──index_jobs/webhook_jobs──▶ PostgreSQL ◀──claims── Indexer/Webhook workers
```

This means worker replicas are horizontally scalable and restart-tolerant. Duplicate active jobs are guarded by database indexes and repository checks. Stale running jobs are recovered by each worker role according to the configured stale threshold and max-attempt settings. If ZincSearch data is lost or suspected stale, rebuild it from PostgreSQL with the reindex admin operation.

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
| Migration Job completed and app/worker pods use `OLLANTA_AUTO_MIGRATE=false` | |
| API, scan worker, indexer, and webhook worker Deployments configured separately | |
| Worker admin `/readyz` probes target each role dependency set | |
| Scan intake queue pressure limits chosen for the database and worker capacity | |
| PostgreSQL accessible from cluster | |
| ZincSearch Deployment + PVC + Service running | |
| ollantaweb Deployment with probes, resources, security context | |
| Service + Ingress with TLS | |
| HPA configured | |
| PodDisruptionBudget set | |
| Network policies (optional — restrict ollantaweb → postgres, ollantaweb → zincsearch) | |
