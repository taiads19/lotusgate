# 🪷 LotusGate

> **O Proxy de Borda para IA Leve e Sem Configuração em Go Puro.**  
> Proxy reverso ultrarrápido, limitador de taxa (token-bucket) e terminação TLS ECDSA autoassinada automática para servidores de LLM e agentes de IA.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://github.com/taiads19/lotusgate/releases)
[![Go 100%](https://img.shields.io/badge/Language-Go%20100%25-00ADD8.svg)](main.go)

---

## ⚡ Por que o LotusGate?

A maioria dos gateways para IA (como Kong, Envoy, LiteLLM) são pesados, exigem ambientes lentos em Python/Node.js ou demandam configurações gigantescas em arquivos YAML apenas para ficarem na frente de um servidor de modelos.

O **LotusGate** foi desenvolvido sob medida para a infraestrutura moderna de IA:
- 🚀 **Zero Dependências Externas:** Construído 100% utilizando apenas a biblioteca padrão do Go (`net/http`, `crypto/tls`). Sem bibliotecas de terceiros vulneráveis.
- 🔐 **Terminação TLS Instantânea:** Gera automaticamente certificados modernos ECDSA P-256 ao iniciar, caso nenhum certificado seja fornecido. Sem necessidade de comandos complexos do OpenSSL.
- ⏱️ **Seguro para Streaming de IA e Long-Polling:** Possui limites de tempo otimizados (30s+ por padrão) que evitam que conexões de streaming contínuo de tokens de LLM (`SSE` / `stream=True`) sejam derrubadas antes da hora.
- 🛡️ **Controle de Taxa por Token-Bucket:** Limitação de requisições por IP diretamente na memória com reposição atômica e segura para concorrência massiva.
- 📦 **Binário Único e Independente:** Um executável estático de apenas ~10 MB que consome menos de 15 MB de memória RAM sob carga de trabalho.

---

## 🏗️ Arquitetura

```
                       ┌──────────────────────────────────────────────┐
                       │                  LotusGate                   │
                       │           (Alta Concorrência em Go)          │
Tráfego de Entrada     │                                              │      Servidor de IA / Backend
HTTP  :8080 ──────────►│  [ Limitador Token-Bucket por IP ]           │      (ex: vLLM, Ollama, FastAPI, ICHYRA)
HTTPS :8443 ──────────►│  [ Terminação TLS Automática (ECDSA) ]       │────► http://127.0.0.1:8000
(Certificado Automático)│  [ Carimbo de Requisição e Rastreabilidade ] │
                       │  [ Proteção Ativa 502 Bad Gateway ]          │
                       └──────────────────────────────────────────────┘
```

---

## 🚀 Como Iniciar Rápido

### 1. Executar com as Configurações Padrão
```bash
# Inicia HTTP na porta :8080 e HTTPS na porta :8443 encaminhando para http://127.0.0.1:8000
./lotusgate
```

### 2. Configurar via Variáveis de Ambiente
Você pode personalizar qualquer porta ou servidor de destino:

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `LISTEN` | `:8080` | Porta de escuta para HTTP simples |
| `TLS_LISTEN` | `:8443` | Porta de escuta para HTTPS seguro |
| `UPSTREAM` | `http://127.0.0.1:8000` | Endereço do seu servidor backend/LLM |
| `RATE_PER_SEC` | `60` | Limite de requisições por segundo por IP |
| `TIMEOUT_SEC` | `30` | Tempo limite de espera da resposta da IA em segundos |
| `TLS_CERT` | `certs/server.crt` | Caminho do arquivo do certificado TLS |
| `TLS_KEY` | `certs/server.key` | Caminho do arquivo da chave privada TLS |

---

## 🧪 Como Rodar os Testes

```bash
go test -v ./...
```

---

## 📄 Licença
Licença MIT. Livre e aberto para a comunidade e uso comercial.
