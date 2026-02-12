# GraphScope

A GraphQL reconnaissance and security testing tool built in Go. Single binary, zero-config, dark-themed web UI.

GraphScope provides schema introspection parsing, interactive type visualization, an MITM proxy for traffic capture, automatic query generation, similarity analysis, and security auditing — all from a single binary with an embedded web interface.

## Features

- **Introspection Parser** — Paste introspection JSON, get full schema analysis
- **Schema Visualization** — Interactive D3.js force-directed graph of type relationships
- **Query Generator** — Auto-build queries/mutations with correct arguments and example values
- **MITM Proxy** — Intercept HTTPS traffic, detect and capture GraphQL operations in real-time
- **Similarity Engine** — Fingerprint, cluster, and compare captured queries structurally
- **Schema Reconstruction** — Infer schema from traffic when introspection is disabled
- **Security Analysis** — Depth analysis, complexity scoring, IDOR detection, dangerous mutation flagging
- **Introspection Bypass** — 11 automated bypass techniques against WAF-protected endpoints
- **Field Fuzzer** — Wordlist-based field discovery via error message mining
- **Schema Diffing** — Compare schema versions, detect breaking changes and privilege escalation

## Quick Start

```bash
# Build
make build

# Run
./graphscope

# With auto-starting proxy
./graphscope -auto-proxy

# Custom ports
./graphscope -addr :9090 -proxy :9999
```

Open `http://localhost:8080` in your browser.

## Usage

### 1. Introspection Mode

Paste your introspection JSON into the dashboard. Supported formats:

```json
{"data":{"__schema":{...}}}
{"__schema":{...}}
{"queryType":{...}, "types":[...]}
```

The schema explorer will show all types, operations, and relationships.

### 2. Proxy Mode

1. Install the CA certificate from `~/.graphscope/ca.pem` into your browser/system trust store
2. Start the proxy from the UI or with `-auto-proxy`
3. Configure your browser/tool to proxy through `:8888`
4. Browse any GraphQL API — requests appear in real-time via SSE

### 3. Query Generation

Click any operation in the schema explorer or generator view. GraphScope will:

- Build a complete query with proper variable definitions
- Fill in context-aware example values (emails, IDs, pagination params)
- Expand nested return types to configurable depth
- Generate inline fragments for unions/interfaces
- Show a ready-to-use cURL command

### 4. Security Analysis

Run the full analysis suite against any parsed schema:

| Module | What It Detects |
|---|---|
| Depth Analysis | Operations with deep nesting (>7 levels) |
| Complexity Estimation | High-cost operations that could enable DoS |
| Dangerous Mutations | delete/admin/resetPassword/grant/execute patterns |
| IDOR Detection | ID-type arguments on queries and mutations |
| Auth Pattern Analysis | Missing auth directives, sensitive operations |
| Introspection Bypass | 11 techniques to bypass disabled introspection |
| Field Fuzzer | Discover valid fields via error message suggestions |
| Schema Diff | Breaking changes, new mutations, privilege escalation |

## Architecture

### System Overview

```mermaid
graph TB
    subgraph "GraphScope Binary"
        direction TB

        subgraph "Web Layer"
            WS[Web Server :8080]
            TM[Template Engine]
            MW[Middleware<br/>Logging + Recovery]
        end

        subgraph "Handler Layer"
            HD[Dashboard]
            HI[Introspection]
            HS[Schema Explorer]
            HG[Query Generator]
            HP[Proxy Control]
            HA[Security Analysis]
        end

        subgraph "Core Services"
            IP[Introspection Parser]
            QG[Query Generator Engine]
            SE[Similarity Engine]
            SR[Schema Reconstructor]
        end

        subgraph "Proxy Engine"
            PR[MITM Proxy :8888]
            CM[Cert Manager]
            GD[GraphQL Detector]
            IC[Traffic Interceptor]
        end

        subgraph "Analysis Engine"
            AD[Depth Analyzer]
            AC[Complexity Estimator]
            AM[Mutation Scanner]
            AI[IDOR Detector]
            AB[Bypass Engine]
            AF[Field Fuzzer]
            ADF[Schema Differ]
        end

        subgraph "Storage"
            REG[Schema Registry<br/>In-Memory Cache]
            DB[(SQLite<br/>WAL Mode)]
        end
    end

    U[User Browser] -->|HTTP| WS
    WS --> MW --> TM
    TM --> HD & HI & HS & HG & HP & HA

    HI --> IP
    HS --> REG
    HG --> QG
    HP --> PR
    HA --> AD & AC & AM & AI & AB & AF & ADF

    IP --> REG
    REG --> DB

    PR --> CM
    PR --> GD
    GD --> IC
    IC --> DB
    IC -->|SSE| WS

    SE --> DB
    SR --> IP

    T[Target GraphQL API] <-->|HTTPS/HTTP| PR
```

### Data Flow Diagram (DFD)

```mermaid
flowchart LR
    subgraph "External"
        USER([User])
        TARGET([Target API])
    end

    subgraph "Level 0"
        GS{{"GraphScope"}}
    end

    USER -->|Introspection JSON| GS
    USER -->|Browse UI| GS
    GS -->|Schema View<br/>Generated Queries<br/>Analysis Reports| USER
    GS <-->|Proxied Traffic| TARGET

    style GS fill:#161b22,stroke:#58a6ff,color:#e6edf3
```

#### Level 1 DFD

```mermaid
flowchart TB
    USER([User])
    TARGET([Target API])

    subgraph "GraphScope Processes"
        P1[1.0<br/>Parse Introspection]
        P2[2.0<br/>Explore Schema]
        P3[3.0<br/>Generate Queries]
        P4[4.0<br/>Proxy Traffic]
        P5[5.0<br/>Analyze Security]
        P6[6.0<br/>Cluster Queries]
        P7[7.0<br/>Reconstruct Schema]
    end

    subgraph "Data Stores"
        D1[(D1: Schemas)]
        D2[(D2: Traffic)]
        D3[(D3: Analysis)]
    end

    USER -->|Raw JSON| P1
    P1 -->|Parsed Schema| D1

    USER -->|View Request| P2
    D1 -->|Schema Data| P2
    P2 -->|Types, Graph, Ops| USER

    USER -->|Select Operation| P3
    D1 -->|Schema| P3
    P3 -->|Query + Variables + cURL| USER

    TARGET <-->|HTTP/HTTPS| P4
    P4 -->|Captured Request| D2
    P4 -->|SSE Event| USER

    USER -->|Run Analysis| P5
    D1 -->|Schema| P5
    P5 -->|Results| D3
    P5 -->|Report| USER

    D2 -->|Traffic| P6
    P6 -->|Clusters| USER

    D2 -->|Traffic| P7
    P7 -->|Inferred Schema| D1

    USER -->|Bypass/Fuzz| P5
    P5 <-->|Probes| TARGET
```

### MITM Proxy Flow

```mermaid
sequenceDiagram
    participant C as Client Browser
    participant P as GraphScope Proxy
    participant CA as Cert Manager
    participant T as Target API
    participant DB as SQLite
    participant UI as Web UI (SSE)

    C->>P: CONNECT target.com:443
    P->>C: 200 Connection Established
    P->>CA: GetCertificate("target.com")
    CA-->>CA: Mint ECDSA cert signed by CA
    CA->>P: TLS Certificate
    P->>C: TLS Handshake (minted cert)

    C->>P: POST /graphql (encrypted)
    P-->>P: Decrypt TLS
    P-->>P: Detect GraphQL request
    P-->>P: Extract query + variables
    P->>T: Forward original request
    T->>P: GraphQL response
    P-->>P: Capture response
    P->>C: Forward response (re-encrypted)

    P->>DB: Store CapturedRequest
    P->>UI: SSE event (new traffic)
```

### Query Generation Flow

```mermaid
flowchart TD
    START([User clicks operation]) --> FIND[Find operation in<br/>root type fields]
    FIND --> ARGS{Has arguments?}

    ARGS -->|Yes| VARDEFS[Build variable definitions<br/>with type signatures]
    VARDEFS --> EXAMPLES[Generate context-aware<br/>example values]
    ARGS -->|No| EXPAND

    EXAMPLES --> EXPAND[Expand return type fields]

    EXPAND --> CHECK{Field type?}
    CHECK -->|Scalar/Enum| LEAF[Include field name]
    CHECK -->|Object| DEPTH{depth < maxDepth?}
    CHECK -->|Union/Interface| FRAG[Generate inline fragments<br/>for each possibleType]

    DEPTH -->|Yes| RECURSE[Recurse into object fields]
    DEPTH -->|No| STOP[Stop expansion]

    FRAG --> RECURSE
    RECURSE --> CHECK

    LEAF --> FORMAT[Format as valid<br/>GraphQL query string]
    STOP --> FORMAT
    FORMAT --> OUTPUT([Return query +<br/>variables + cURL])
```

### Security Analysis Pipeline

```mermaid
flowchart LR
    SCHEMA([Parsed Schema]) --> SPLIT{Analysis Modules}

    SPLIT --> DEPTH[Depth Analyzer<br/>Walk type tree<br/>Track max nesting]
    SPLIT --> COMPLEX[Complexity Estimator<br/>Count fields<br/>Multiply by list depth]
    SPLIT --> MUTANT[Mutation Scanner<br/>Pattern match names<br/>delete/admin/grant...]
    SPLIT --> IDOR[IDOR Detector<br/>Find ID-type args<br/>on queries/mutations]
    SPLIT --> AUTH[Auth Analyzer<br/>Check directives<br/>Flag sensitive ops]

    DEPTH --> RESULTS([Analysis Report<br/>with severity ratings])
    COMPLEX --> RESULTS
    MUTANT --> RESULTS
    IDOR --> RESULTS
    AUTH --> RESULTS

    RESULTS --> DB[(SQLite)]
    RESULTS --> UI([Web UI])
```

## Directory Structure

```
graphscope/
├── cmd/graphscope/         # Entry point, flag parsing, wiring
│   └── main.go
├── internal/
│   ├── server/             # HTTP server, stdlib router, middleware
│   ├── handler/            # Request handlers (dashboard, schema, proxy, analysis)
│   ├── parser/             # Introspection JSON parser, query parser, schema reconstruction
│   ├── schema/             # Core data models, registry, type resolver, graph builder
│   ├── generator/          # Query generation, variable examples, depth/complexity analysis
│   ├── proxy/              # MITM proxy engine, ECDSA cert manager, GraphQL detection
│   ├── similarity/         # Query fingerprinting, clustering, Jaccard similarity
│   ├── analysis/           # Security modules (mutations, IDOR, bypass, fuzzer, diff)
│   ├── storage/            # SQLite setup, migrations, repos (schema, traffic, analysis)
│   └── wordlist/           # Embedded field wordlist for fuzzing
├── web/
│   ├── embed.go            # embed.FS declarations
│   ├── templates/          # Go HTML templates + HTMX partials
│   └── static/             # CSS, JS, vendored D3.js + HTMX
├── go.mod
├── Makefile
└── README.md
```

## Tech Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.22+ | Stdlib router, single binary, goroutines |
| Web Framework | `net/http` stdlib | Zero dependencies, method routing since Go 1.22 |
| Frontend | Go templates + HTMX + D3.js | No build tooling, vendored JS, server-rendered |
| Database | SQLite (WAL mode) | Embedded, zero-config, portable |
| TLS/Crypto | `crypto/x509` + `crypto/ecdsa` | Stdlib, ECDSA P-256 for fast cert minting |
| Visualization | D3.js v7 force-directed graph | Custom schema graph, no graphql-voyager |

**External dependencies: 1** — `github.com/mattn/go-sqlite3`

Everything else is Go standard library or vendored JS.

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8080` | Web UI listen address |
| `-proxy` | `:8888` | MITM proxy listen address |
| `-db` | `~/.graphscope/graphscope.db` | SQLite database path |
| `-auto-proxy` | `false` | Start proxy automatically on launch |

## Files Generated

On first run, GraphScope creates `~/.graphscope/` containing:

| File | Purpose |
|---|---|
| `ca.pem` | CA certificate — install in browser/system trust store for HTTPS interception |
| `ca-key.pem` | CA private key (ECDSA P-256) — kept with 0600 permissions |
| `graphscope.db` | SQLite database for schemas, traffic, and analysis results |

## Security Considerations

- The MITM proxy uses `InsecureSkipVerify` when forwarding to targets — this is **by design** for a security testing tool. Do not use in production environments.
- The web UI has **no authentication**. Bind to `localhost` or use in isolated networks only.
- CA private key is stored at `~/.graphscope/ca-key.pem` with restricted permissions. Protect this file.
- The field fuzzer and bypass engine send HTTP requests to external targets. Use only against systems you are authorized to test.

## License

For authorized security testing, defensive security research, and educational use only.
