# Distributed API Gateway + Rate Limiter

A production-shaped API gateway written in Go that sits in front of backend services, authenticates clients with API keys, enforces configurable per-plan rate limits, and exposes Prometheus/Grafana observability for live traffic analysis.

The project is designed around a real infrastructure problem: rate limits must hold even when the gateway is horizontally scaled. A naive in-memory counter works for one process, but fails once traffic is load-balanced across multiple gateway instances. This implementation supports both local and Redis-backed limiters so the distributed behavior can be tested, compared, and demonstrated.

## What It Does

This gateway accepts incoming HTTP requests, authenticates them using `X-API-Key`, applies the configured rate-limit policy for the authenticated principal, and proxies allowed requests to an upstream backend service.

Rejected requests return `429 Too Many Requests` with rate-limit headers, while successful requests are forwarded without exposing the raw API key to the upstream service.

At a high level:

```text
Client
  |
  v
Nginx Load Balancer
  |
  +--> Gateway Instance 1
  +--> Gateway Instance 2...
  +--> Gateway Instance n
          |
          v
        Redis
   shared distributed
   rate-limit state
          |
          v
   Mock Backend API
```

## Core Features

* API key authentication using SHA-256 hashed API key records
* Per-principal rate limiting based on stable authenticated identity, not raw secrets
* Configurable rate-limit policies by plan, such as `free`, `pro`, or `enterprise`
* Multiple limiter algorithms:
  * Fixed window counter
  * Token bucket
  * Sliding window log
* In-memory limiter implementations for local algorithm testing
* Redis-backed limiter implementations for distributed gateway deployments
* Reverse proxy behavior using Go’s standard HTTP reverse proxy
* Horizontal gateway deployment behind Nginx
* Prometheus metrics for request volume, rejection rate, latency, and in-flight requests
* Grafana dashboard for live gateway visibility
* Docker Compose environment with Redis, Nginx, Prometheus, Grafana, three gateway instances, and a mock backend

## Architecture

```text
                  ┌────────────────────┐
                  │      Client        │
                  └─────────┬──────────┘
                            │
                            v
                  ┌────────────────────┐
                  │       Nginx        │
                  │  Load Balancer     │
                  └─────────┬──────────┘
                            │
        ┌───────────────────┼───────────────────┐
        v                   v                   v
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ Gateway 1     │   │ Gateway 2     │   │ Gateway 3     │
│ Auth          │   │ Auth          │   │ Auth          │
│ Rate Limit    │   │ Rate Limit    │   │ Rate Limit    │
│ Proxy         │   │ Proxy         │   │ Proxy         │
└───────┬───────┘   └───────┬───────┘   └───────┬───────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            v
                    ┌──────────────┐
                    │    Redis     │
                    │ Shared State │
                    └──────┬───────┘
                           │
                           v
                    ┌──────────────┐
                    │ Mock Backend │
                    └──────────────┘
```

## Technology Stack

| Area                | Technology              |
| ------------------- | ----------------------- |
| Gateway service     | Go                      |
| Reverse proxy       | `net/http/httputil`     |
| Distributed state   | Redis                   |
| Load balancing      | Nginx                   |
| Metrics             | Prometheus              |
| Dashboards          | Grafana                 |
| Local orchestration | Docker Compose          |

## Rate Limiting Algorithms

The gateway includes multiple limiter implementations so their behavior can be compared under the same interface.

| Algorithm          | Best For                                 | Trade-off                         |
| ------------------ | ---------------------------------------- | --------------------------------- |
| Fixed Window       | Simple quotas per time window            | Allows boundary bursts            |
| Token Bucket       | Bursty clients with steady refill        | Slightly more complex state model |
| Sliding Window Log | More accurate rolling-window enforcement | Higher storage cost per key       |

Each algorithm has an in-memory implementation for simple local testing and a Redis-backed implementation for distributed gateway deployments.

## Observability

The gateway exposes Prometheus metrics at `/metrics`, including:

* Total HTTP requests by method, path, and status
* Rate-limit rejection count
* Request duration histogram
* Current in-flight requests

The included Grafana dashboard visualizes:

* Request rate
* Rejection rate
* p95 and p99 latency
* Requests by gateway instance
* In-flight requests

## Local Demo

The local Docker Compose setup runs:

* 3 gateway instances
* 1 Nginx load balancer
* 1 Redis instance
* 1 mock backend API
* Prometheus
* Grafana

The default gateway entrypoint is:

```text
http://localhost:8080
```

The mock backend exposes:

```text
GET /api/hello
```

Prometheus is available at:

```text
http://localhost:9090
```

Grafana is available at:

```text
http://localhost:3000
```

## Project Status

Implemented:

* API key authentication
* Reverse proxy gateway
* Fixed window, token bucket, and sliding window limiters
* In-memory and Redis-backed limiter variants
* Per-plan policy parsing
* Multi-instance Docker Compose deployment
* Nginx load balancing
* Prometheus metrics
* Grafana dashboard provisioning
* Mock backend service
* Unit tests for limiter behavior

Planned or production-hardening candidates:

* Admin API for key and policy management
* Persistent API key storage
* Request tracing
* Structured logging
* mTLS or JWT support
* Dynamic config reloads
* Multi-upstream routing rules
* Kubernetes deployment manifests
* Load-test reports and benchmark documentation


## Related Documentation

Start here:

* `docs/architecture.md` — system design and request flow
* `docs/configuration.md` — environment variables and runtime modes
* `docs/limiter-algorithms.md` — fixed window vs. token bucket vs. sliding window
* `docs/distributed-testing.md` — validating limits across multiple gateway instances
* `docs/observability.md` — Prometheus metrics and Grafana dashboard
* `docs/production-readiness.md` — what would be needed for a real production gateway
