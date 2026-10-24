# 🪷 LotusGate

> **The Lightweight, Zero-Config AI Edge Proxy in Pure Go.**  
> Ultra-fast reverse proxy, token-bucket rate limiter, and automatic self-signed ECDSA TLS termination for LLM backends and AI agents.

[![Go Report Card](https://goreportcard.com/badge/github.com/ichyra/lotusgate)](https://github.com/ichyra/lotusgate)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://github.com/ichyra/lotusgate/releases)

---

## ⚡ Why LotusGate?

Most AI gateways (Kong, Envoy, LiteLLM) are either bloated, require Python/Node runtimes, or demand complex YAML configurations just to sit in front of a model server.

**LotusGate** is engineered for modern AI infrastructures:
- 🚀 **Zero Third-Party Dependencies:** Built 100% on Go's standard library (`net/http`, `crypto/tls`).
- 🔐 **Instant TLS Termination:** Automatically generates ECDSA P-256 certificates on startup if none are provided. No OpenSSL commands required.
- ⏱️ **AI Streaming & Long-Polling Safe:** Optimized timeouts (30s+ default) that prevent prematurely cutting off streaming LLM tokens (`SSE` / `stream=True`).
- 🛡️ **Token-Bucket Rate Limiting:** In-memory, per-IP rate limiting with thread-safe atomic replenishments.
- 📦 **Single Standalone Binary:** ~10 MB static binary consuming under 15 MB of RAM under load.

---

## 🏗️ Architecture

```
                       ┌──────────────────────────────────────────────┐
                       │                  LotusGate                   │
                       │           (High-Concurrency in Go)           │
Incoming Traffic       │                                              │      Upstream AI Engine
HTTP  :8080 ──────────►│  [ Token-Bucket Limiter ]                    │      (e.g., ICHYRA Core / FastAPI / vLLM)
HTTPS :8443 ──────────►│  [ TLS Termination (ECDSA) ]                │────► http://127.0.0.1:8000
 (Auto-Cert)           │  [ Request ID & Tracing ]                    │
                       │  [ 502 Sentinel Shield ]                     │
                       └──────────────────────────────────────────────┘
```

---

## 🚀 Quick Start

### 1. Run with Default Settings
```bash
# Starts HTTP on :8080 and HTTPS on :8443 proxying to http://127.0.0.1:8000
./lotusgate
```

### 2. Configure via Environment Variables
| Variable | Default | Description |
| :--- | :--- | :--- |
| `LISTEN` | `:8080` | Plain HTTP listening port |
| `TLS_LISTEN` | `:8443` | Native HTTPS listening port |
| `UPSTREAM` | `http://127.0.0.1:8000` | Target backend/LLM server |
| `RATE_PER_SEC` | `60` | Requests per second allowed per IP |
| `TIMEOUT_SEC` | `30` | Upstream backend response timeout |
| `TLS_CERT` | `certs/server.crt` | Path to TLS certificate |
| `TLS_KEY` | `certs/server.key` | Path to TLS private key |

---

## 🧪 Testing

```bash
go test -v ./...
```

---

## 📄 License
MIT License. Open for the community.
