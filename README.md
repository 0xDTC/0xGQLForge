# 0xGQLForge

A GraphQL reconnaissance and security testing tool built in Go. Single binary, zero-config, dark-themed web UI.

0xGQLForge provides schema introspection parsing, interactive type visualization, an MITM proxy for traffic capture with **live schema inference from response bodies**, automatic query generation, similarity analysis, and security auditing — all from a single binary with an embedded web interface.

## Features

- **Introspection Parser** — Paste introspection JSON, get full schema analysis
- **Schema Visualization** — Interactive D3.js force-directed graph of type relationships
- **Query Generator** — Auto-build queries/mutations with correct arguments and example values
- **MITM Proxy** — Intercept HTTPS traffic, detect and capture GraphQL operations in real-time via SSE
- **Proxy Projects** — Organize captured traffic into named projects, link inferred schemas to each
- **Schema Inference** — Parse response bodies to reconstruct real object types and graph edges; auto-detect introspection responses for instant full schemas
- **Similarity Engine** — Fingerprint, cluster, and compare captured queries structurally
- **Security Analysis** — Depth analysis, complexity scoring, IDOR detection, dangerous mutation flagging
- **Introspection Bypass** — 11 automated bypass techniques against WAF-protected endpoints
- **Field Fuzzer** — Wordlist-based field discovery via error message mining
- **Schema Diffing** — Compare schema versions, detect breaking changes and privilege escalation

## Quick Start

```bash
# Build (requires CGO for SQLite)
make build

# Run with defaults (UI: :8080, Proxy: :8888)
./gqlforge

# Start proxy automatically on launch
./gqlforge -auto-proxy

# Custom ports
./gqlforge -addr :9090 -proxy :9999
```

Open `http://localhost:8080` in your browser.

## Usage

### 1. Introspection Mode

Navigate to **Schemas** (home page). Paste your introspection JSON. Supported formats:

```json
{"data":{"__schema":{...}}}
{"__schema":{...}}
{"queryType":{...}, "types":[...]}
```

The schema explorer shows all types, operations, and relationships. From there open the **Graph** (D3.js force-directed visualization) or the **Generator**.

### 2. Proxy Mode + Schema Inference

The proxy is a MITM HTTP/HTTPS interceptor. It captures only GraphQL traffic and streams it live to the UI via SSE — no page refresh needed.

**Setup:**
1. Install `~/.gqlforge/ca.pem` into your browser/system trust store
2. Go to **Proxy** in the nav bar and click **Start Proxy**
3. Configure your browser or tool to use `127.0.0.1:8888` as the proxy
4. Browse any GraphQL API — requests appear in the traffic table in real-time

**Link to a Project:**
- Create a project on the **Projects** page first
- On the **Proxy** page, select the project from the dropdown and click **Apply**
- All subsequent captured traffic is tagged to that project

**Build a Schema from Traffic:**
- Go to **Projects → [your project] → Build Schema from Traffic**
- The inference engine walks captured response bodies to discover real object types:
  - `{"data":{"user":{"id":"1","name":"Alice","posts":[...]}}}` → creates `User` and `Post` types with edges
  - `id` / `*Id` fields → `ID` scalar; booleans → `Boolean`; numbers → `Int` / `Float`
  - Arrays of objects → `[TypeName]` list references with automatic singularization
- If an introspection query was made through the proxy, the full schema is extracted automatically from the response — no manual steps needed

**Schema Grows Over Time:**
The more endpoints you browse, the richer the graph becomes. Each captured response adds new types or merges new fields into existing types.

### 3. Query Generation

Click any operation in the schema explorer or generator view. 0xGQLForge will:

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

---

## Architecture

### System Overview

```mermaid
graph TB
    subgraph "0xGQLForge Binary"
        direction TB

        subgraph "Web Layer"
            WS[Web Server :8080]
            MW[Middleware<br/>Logging + Recovery]
            TM[Template Engine<br/>Go html/template]
        end

        subgraph "Handler Layer"
            HSL[Schemas]
            HI[Introspection]
            HSV[Schema Explorer]
            HG[Query Generator]
            HP[Proxy Control]
            HPJ[Projects]
            HA[Security Analysis]
        end

        subgraph "Core Services"
            IP[Introspection Parser]
            QG[Query Generator Engine]
            SE[Similarity Engine]
            INF[Inference Engine<br/>Response-Body Typing]
        end

        subgraph "Proxy Engine"
            PR[MITM Proxy :8888]
            CM[Cert Manager<br/>ECDSA P-256]
            GD[GraphQL Detector]
            SSE[SSE Pub/Sub]
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

        subgraph "Storage — SQLite WAL"
            SR[(SchemaRepo)]
            TR[(TrafficRepo<br/>+ response_body)]
            AR[(AnalysisRepo)]
            PJR[(ProjectRepo)]
        end
    end

    U[User Browser] -->|HTTP| WS
    WS --> MW --> TM
    TM --> HSL & HI & HSV & HG & HP & HPJ & HA

    HI --> IP --> SR
    HSV --> SR
    HG --> QG
    HP --> PR
    HP -->|SSE stream| SSE
    HPJ --> PJR
    HPJ --> INF
    INF -->|reads response bodies| TR
    INF -->|saves inferred schema| SR

    HA --> AD & AC & AM & AI & AB & AF & ADF

    PR --> CM & GD
    GD -->|store request + response| TR
    GD --> SSE -->|live event| U

    T[Target GraphQL API] <-->|HTTP/HTTPS| PR
```

### Data Flow

```mermaid
flowchart TB
    USER([Analyst])
    TARGET([Target GraphQL API])

    subgraph "Ingestion"
        P1[Parse Introspection JSON]
        P4[MITM Proxy Capture]
    end

    subgraph "Analysis & Generation"
        P2[Schema Explorer + Graph]
        P3[Query Generator]
        P5[Security Analysis]
        P6[Similarity Clustering]
        P7[Schema Inference]
    end

    subgraph "Storage"
        D1[(Schemas)]
        D2[(Traffic +<br/>Response Bodies)]
        D3[(Analysis Results)]
        D4[(Projects)]
    end

    USER -->|Paste introspection JSON| P1
    P1 -->|Parsed schema| D1

    USER -->|Explore| P2
    D1 --> P2
    P2 -->|Type graph, ops, fields| USER

    USER -->|Select operation| P3
    D1 --> P3
    P3 -->|Query + variables + cURL| USER

    TARGET <-->|HTTP/HTTPS MITM| P4
    P4 -->|Request + full response body| D2
    P4 -->|SSE live event| USER

    USER -->|Create project, link proxy| D4

    USER -->|Build Schema from Traffic| P7
    D2 -->|Requests with response bodies| P7
    D4 --> P7
    P7 -->|Inferred schema with real types + edges| D1

    USER -->|Run analysis| P5
    D1 --> P5
    P5 --> D3
    P5 -->|Report| USER

    D2 --> P6
    P6 -->|Clusters + fingerprints| USER
```

### MITM Proxy Flow

```mermaid
sequenceDiagram
    participant C as Client Browser
    participant P as Proxy Engine
    participant CA as Cert Manager
    participant T as Target API
    participant DB as SQLite
    participant UI as Web UI (SSE)

    C->>P: CONNECT target.com:443
    P->>C: 200 Connection Established
    P->>CA: GetCertificate("target.com")
    CA-->>CA: Mint ECDSA cert signed by local CA
    CA->>P: TLS Certificate
    P->>C: TLS Handshake with minted cert

    loop Per GraphQL request
        C->>P: POST /graphql {"query":"...","variables":{}}
        P-->>P: Detect GraphQL (path / Content-Type / ?query=)
        P-->>P: Extract query, operationName, variables
        P->>T: Forward original request
        T->>P: GraphQL response {"data":{...}}
        P->>C: Forward response to client

        P->>DB: Store CapturedRequest incl. full response_body
        P->>UI: SSE data event → new row in traffic table
    end
```

### Schema Inference Flow

```mermaid
flowchart TD
    START([Build Schema from Traffic]) --> LOAD[Load all captured requests<br/>including response bodies]

    LOAD --> INTRO{Any response body<br/>contains __schema?}

    INTRO -->|Yes — introspection response| FULL[parser.ParseIntrospection<br/>Full schema extracted instantly]
    FULL --> SAVE

    INTRO -->|No| WALK[Walk each response body]

    WALK --> DATA{"Has data field?"}
    DATA -->|No / null| SKIP[Skip — no type info]
    DATA -->|Yes| FIELDS[Iterate top-level fields<br/>→ root Query/Mutation fields]

    FIELDS --> VAL{Field value shape?}
    VAL -->|string — id field| IDS[TypeRef: ID scalar]
    VAL -->|string — other| STR[TypeRef: String scalar]
    VAL -->|boolean| BOOL[TypeRef: Boolean scalar]
    VAL -->|integer| INT[TypeRef: Int scalar]
    VAL -->|float| FLT[TypeRef: Float scalar]
    VAL -->|null| UNK[TypeRef: String — unknown]
    VAL -->|JSON object| OBJ["Create Object type<br/>name = PascalCase(field)<br/>Recurse sub-fields"]
    VAL -->|array of objects| LIST["Create List of Object type<br/>name = PascalCase(singularize(field))<br/>Recurse first element"]
    VAL -->|array of scalars| SLST[Create List of scalar type]

    OBJ & LIST --> MERGE{Type name seen before?}
    MERGE -->|Yes| MRG[Merge fields — union of all<br/>fields seen across all responses]
    MERGE -->|No| NEW[Register new type]

    IDS & STR & BOOL & INT & FLT & UNK & MRG & NEW & SLST --> ROOT[Add field to root type<br/>Query / Mutation / Subscription]

    ROOT --> BUILD[Assemble schema.Schema<br/>with all discovered types]
    BUILD --> SAVE[Save to SchemaRepo<br/>Link schema ID to Project]
    SAVE --> GRAPH([Graph now shows real nodes + edges<br/>that grow with each new request])
```

### Query Generation Flow

```mermaid
flowchart TD
    START([User selects operation]) --> FIND[Resolve operation in root type]
    FIND --> ARGS{Has arguments?}

    ARGS -->|Yes| VARDEFS[Build $var: Type! definitions]
    VARDEFS --> EXAMPLES[Generate example values<br/>email / ID / int / bool / enum]
    ARGS -->|No| EXPAND

    EXAMPLES --> EXPAND[Expand return type fields]

    EXPAND --> CHECK{Field kind?}
    CHECK -->|Scalar / Enum| LEAF[Emit field name]
    CHECK -->|Object| DEPTH{depth < maxDepth?}
    CHECK -->|Union / Interface| FRAG[Inline fragments per possibleType]

    DEPTH -->|Yes| RECURSE[Recurse into object fields]
    DEPTH -->|No| STOP[Stop — depth limit reached]

    FRAG --> RECURSE
    RECURSE --> CHECK
    LEAF --> FORMAT[Format valid GraphQL string]
    STOP --> FORMAT
    FORMAT --> OUTPUT([Query + variables JSON + cURL command])
```

### Security Analysis Pipeline

```mermaid
flowchart LR
    SCHEMA([Parsed Schema]) --> SPLIT{Dispatch to modules}

    SPLIT --> DEPTH[Depth Analyzer<br/>Walk type tree<br/>Track max nesting]
    SPLIT --> COMPLEX[Complexity Estimator<br/>Fields × list depth<br/>low / med / high / critical]
    SPLIT --> MUTANT[Mutation Scanner<br/>Pattern match names<br/>delete / admin / grant / exec]
    SPLIT --> IDOR[IDOR Detector<br/>ID args on queries<br/>sequential / uuid / encoded]
    SPLIT --> AUTH[Auth Analyzer<br/>@auth directives<br/>Sensitive operation flags]

    DEPTH & COMPLEX & MUTANT & IDOR & AUTH --> RESULTS([Report with severity ratings])
    RESULTS --> DB[(SQLite — AnalysisRepo)]
    RESULTS --> UI([Analysis View])
```

---

## Directory Structure

```
0xGQLForge/
├── cmd/gqlforge/
│   └── main.go                  # Entry point: flags, service wiring, graceful shutdown
├── internal/
│   ├── server/                  # HTTP server, routes.go, middleware (recovery + logging)
│   ├── handler/                 # Request handlers: schemas, proxy, projects, analysis
│   ├── parser/                  # Introspection JSON parser (3 formats), query parser
│   ├── schema/                  # Core models (Schema, Type, TypeRef, Field), graph builder
│   ├── generator/               # Query building, variable examples, depth/complexity
│   ├── proxy/                   # MITM engine, ECDSA cert minting, GraphQL detection, SSE pub/sub
│   ├── inference/               # Schema inference from response bodies; introspection auto-detect
│   ├── similarity/              # Query fingerprinting, Jaccard similarity, clustering
│   ├── analysis/                # Security modules: mutations, IDOR, bypass, fuzzer, diff
│   ├── storage/                 # SQLite WAL, migrations, repos (Schema, Traffic, Analysis, Project)
│   └── wordlist/                # Embedded field wordlist for fuzzing
├── web/
│   ├── embed.go                 # embed.FS declarations
│   ├── templates/
│   │   ├── layout.html          # Top navbar (Schemas | Projects | Proxy | theme toggle)
│   │   ├── schemas.html         # Home: schema list + introspection upload
│   │   ├── schema.html          # Schema explorer (types, operations, sidebar)
│   │   ├── graph.html           # D3.js force-directed type graph
│   │   ├── generator.html       # Query/mutation builder + cURL
│   │   ├── proxy.html           # Live traffic table (SSE) + inline filters + project link
│   │   ├── projects.html        # Project list with create/delete
│   │   ├── project_detail.html  # Per-project traffic + schema inference trigger
│   │   └── analysis.html        # Security analysis dashboard
│   └── static/
│       ├── css/app.css          # Dark theme design system
│       └── js/                  # D3.js v7, HTMX, app.js
├── go.mod
├── Makefile
└── README.md
```

---

## Tech Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.22+ | Stdlib router, single binary, goroutines for proxy concurrency |
| Web Framework | `net/http` stdlib | Method + path routing since Go 1.22, zero dependencies |
| Frontend | Go templates + HTMX + D3.js | No build tooling, server-rendered HTML, vendored JS |
| Database | SQLite (WAL mode) | Embedded, zero-config, portable, single-file persistence |
| TLS / Crypto | `crypto/x509` + `crypto/ecdsa` | ECDSA P-256, fast per-host cert minting |
| Graph Viz | D3.js v7 force-directed | Custom schema ERD with click/hover lineage highlighting |
| Live Updates | Server-Sent Events (SSE) | Lightweight server push without WebSocket overhead |

**External dependencies: 1** — `github.com/mattn/go-sqlite3` (CGO, C SQLite binding)

Everything else is Go standard library or vendored JS.

---

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8080` | Web UI listen address |
| `-proxy` | `:8888` | MITM proxy listen address |
| `-db` | `~/.gqlforge/gqlforge.db` | SQLite database path |
| `-auto-proxy` | `false` | Start proxy automatically on launch |

## Runtime Files

On first run, 0xGQLForge creates `~/.gqlforge/` containing:

| File | Purpose |
|---|---|
| `ca.pem` | CA certificate — install in browser/system trust store for HTTPS interception |
| `ca-key.pem` | CA private key (ECDSA P-256) — stored with 0600 permissions |
| `gqlforge.db` | SQLite database: schemas, traffic (incl. response bodies), projects, analysis results |

## GraphQL Request Detection

The proxy flags a request as GraphQL if **any** of these match:

| Condition | Example |
|---|---|
| URL path contains `graphql` or `gql` | `/graphql`, `/api/gql`, `/v1/graphql` |
| `GET` with `?query=` parameter | `GET /api?query={user{id}}` |
| `POST` with `Content-Type: application/json` | Any JSON POST — recorded only if `query` field is present |

Supported body shapes: single `{"query":"...","operationName":"...","variables":{}}` or batch `[{"query":"..."},...]` (first item used).

## Security Considerations

- The MITM proxy uses `InsecureSkipVerify` when forwarding to targets — **by design** for a security testing tool. Do not use in production environments.
- The web UI has **no authentication**. Bind to `localhost` or an isolated network only.
- The CA private key at `~/.gqlforge/ca-key.pem` has restricted permissions. Protect this file — anyone with it can impersonate any HTTPS site to browsers that trust your CA.
- The field fuzzer and bypass engine send HTTP requests to external targets. Use only against systems you are authorized to test.

## License

For authorized security testing, defensive security research, and educational use only.
