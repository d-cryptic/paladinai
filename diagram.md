# 📊 Paladin AI Architecture Diagrams

This document contains all the Mermaid diagrams from the Paladin AI documentation, organized by component and functionality.

## System Architecture

### High-Level Architecture
```mermaid
graph TB
    subgraph "Client Layer"
        WEB[Web Frontend<br/>Next.js + React]
        CLI[CLI Client<br/>Python + Rich]
        API_CLIENT[API Client<br/>HTTP/REST]
    end
    
    subgraph "API Gateway"
        GATEWAY[FastAPI Gateway<br/>Authentication & Routing]
    end
    
    subgraph "Core Services"
        SERVER[Paladin Server<br/>LangGraph + FastAPI]
        WORKFLOW[Workflow Engine<br/>LangGraph Orchestration]
        MEMORY[Memory Service<br/>Mem0AI Integration]
        RAG[RAG Service<br/>Document Processing]
    end
    
    subgraph "AI Layer"
        LLM[OpenAI GPT-4]
        EMBEDDINGS[OpenAI Embeddings]
        REASONING[LangGraph Agents]
    end
    
    subgraph "Data Layer"
        MONGO[(MongoDB<br/>Checkpoints & Sessions)]
        VECTOR[(Qdrant<br/>Vector Embeddings)]
        GRAPH[(Neo4j<br/>Memory Graph)]
        CACHE[(Valkey<br/>Redis Compatible)]
    end
    
    subgraph "Monitoring Integration"
        PROM[Prometheus<br/>Metrics]
        LOKI[Loki<br/>Logs]
        GRAFANA[Grafana<br/>Dashboards]
        ALERT[Alertmanager<br/>Alerts]
    end
    
    subgraph "External Services"
        DISCORD[Discord Bot<br/>MCP Protocol]
        WEBHOOK[Webhook Server<br/>External Integrations]
        TOOLS[MCP Tools<br/>Monitoring Connectors]
    end
    
    WEB --> GATEWAY
    CLI --> GATEWAY
    API_CLIENT --> GATEWAY
    
    GATEWAY --> SERVER
    SERVER --> WORKFLOW
    SERVER --> MEMORY
    SERVER --> RAG
    
    WORKFLOW --> LLM
    WORKFLOW --> EMBEDDINGS
    WORKFLOW --> REASONING
    
    SERVER --> MONGO
    RAG --> VECTOR
    MEMORY --> GRAPH
    SERVER --> CACHE
    
    TOOLS --> PROM
    TOOLS --> LOKI
    TOOLS --> GRAFANA
    TOOLS --> ALERT
    
    DISCORD --> SERVER
    WEBHOOK --> SERVER
```

## Deployment Architecture

### Container Orchestration
```mermaid
graph TB
    subgraph "Frontend Layer"
        LB[Load Balancer<br/>Nginx]
        UI[Next.js Frontend<br/>Port 3000]
    end
    
    subgraph "Application Layer"
        API[FastAPI Server<br/>Port 8000]
        CLI[CLI Client]
        DISCORD[Discord Bot MCP]
        WEBHOOK[Webhook Server<br/>Port 8001]
    end
    
    subgraph "Data Layer"
        MONGO[(MongoDB<br/>Checkpoints)]
        QDRANT[(Qdrant<br/>Vector DB)]
        NEO4J[(Neo4j<br/>Graph DB)]
        VALKEY[(Valkey<br/>Cache/Queue)]
    end
    
    subgraph "Monitoring Stack"
        PROM[Prometheus<br/>Port 9090]
        GRAFANA[Grafana<br/>Port 3001]
        LOKI[Loki<br/>Port 3100]
        ALERT[Alertmanager<br/>Port 9093]
    end
    
    subgraph "External Services"
        OPENAI[OpenAI API]
        LANGFUSE[Langfuse]
    end
    
    LB --> UI
    UI --> API
    CLI --> API
    DISCORD --> API
    WEBHOOK --> API
    
    API --> MONGO
    API --> QDRANT
    API --> NEO4J
    API --> VALKEY
    
    PROM --> API
    PROM --> MONGO
    PROM --> QDRANT
    GRAFANA --> PROM
    GRAFANA --> LOKI
    ALERT --> WEBHOOK
    
    API --> OPENAI
    API --> LANGFUSE
```


## Discord Integration

### Discord Bot Architecture
```mermaid
graph TB
    subgraph "Discord Server"
        CHANNELS[Discord Channels]
        THREADS[Auto-Created Threads]
        UPLOADS[Document Uploads]
        MENTIONS[Bot Mentions]
    end
    
    subgraph "Discord MCP Server"
        BOT[Discord Bot]
        PROCESSOR[Message Processor]
        QUEUE[Message Queue]
        WORKERS[Background Workers]
    end
    
    subgraph "Integration Layer"
        GUARDRAIL[AI Guardrails]
        CONTEXT[Context Tracker]
        PDF[PDF Generator]
        ALERT_SERVER[Alert Server]
    end
    
    subgraph "Paladin Core"
        CHAT_API[Chat API]
        MEMORY_API[Memory API]
        DOC_API[Document API]
        ALERT_API[Alert API]
    end
    
    subgraph "External Systems"
        ALERTMANAGER[Alertmanager]
        WEBHOOKS[Webhook Sources]
        MONITORING[Monitoring Stack]
    end
    
    CHANNELS --> BOT
    MENTIONS --> PROCESSOR
    UPLOADS --> BOT
    
    BOT --> QUEUE
    PROCESSOR --> WORKERS
    QUEUE --> GUARDRAIL
    
    GUARDRAIL --> CONTEXT
    CONTEXT --> PDF
    
    BOT --> CHAT_API
    WORKERS --> MEMORY_API
    BOT --> DOC_API
    ALERT_SERVER --> ALERT_API
    
    ALERTMANAGER --> ALERT_SERVER
    WEBHOOKS --> ALERT_SERVER
    MONITORING --> ALERTMANAGER
```

## Workflows

### Workflow Engine Architecture
```mermaid
graph TB
    subgraph "Workflow Engine"
        ENTRY[Entry Point]
        GUARDRAIL[Guardrail Node]
        CATEGORIZE[Categorization Node]
        
        subgraph "Workflow Types"
            QUERY[Query Workflow]
            ACTION[Action Workflow]
            INCIDENT[Incident Workflow]
        end
        
        subgraph "Data Collection"
            PROMETHEUS[Prometheus Node]
            LOKI[Loki Node]
            ALERTMANAGER[Alertmanager Node]
        end
        
        RESULT[Result Node]
        ERROR[Error Handler]
    end
    
    subgraph "Integration Layer"
        MEMORY[Memory System]
        RAG[RAG System]
        TOOLS[MCP Tools]
        CHECKPOINT[Checkpointing]
    end
    
    ENTRY --> GUARDRAIL
    GUARDRAIL --> CATEGORIZE
    CATEGORIZE --> QUERY
    CATEGORIZE --> ACTION
    CATEGORIZE --> INCIDENT
    
    QUERY --> PROMETHEUS
    ACTION --> PROMETHEUS
    INCIDENT --> PROMETHEUS
    
    QUERY --> LOKI
    ACTION --> LOKI
    INCIDENT --> LOKI
    
    QUERY --> ALERTMANAGER
    ACTION --> ALERTMANAGER
    INCIDENT --> ALERTMANAGER
    
    PROMETHEUS --> RESULT
    LOKI --> RESULT
    ALERTMANAGER --> RESULT
    
    GUARDRAIL -.-> ERROR
    CATEGORIZE -.-> ERROR
    PROMETHEUS -.-> ERROR
    LOKI -.-> ERROR
    ALERTMANAGER -.-> ERROR
    
    GUARDRAIL <--> MEMORY
    PROMETHEUS <--> TOOLS
    LOKI <--> TOOLS
    ALERTMANAGER <--> TOOLS
    
    INCIDENT <--> RAG
    
    ENTRY <--> CHECKPOINT
    RESULT <--> CHECKPOINT
```

### Query Workflow Data Flow
```mermaid
sequenceDiagram
    participant User
    participant Categorizer
    participant QueryNode
    participant DataSource
    participant Result
    
    User->>Categorizer: "Is API healthy?"
    Categorizer->>QueryNode: Route to Query workflow
    QueryNode->>DataSource: Collect metrics/alerts
    DataSource->>Result: Return status data
    Result->>User: "API is healthy - 99.9% uptime"
```

### Action Workflow Data Flow
```mermaid
sequenceDiagram
    participant User
    participant Categorizer
    participant ActionNode
    participant MultipleDataSources
    participant Analysis
    participant Result
    
    User->>Categorizer: "Analyze performance trends"
    Categorizer->>ActionNode: Route to Action workflow
    ActionNode->>MultipleDataSources: Collect historical data
    MultipleDataSources->>Analysis: Process and analyze
    Analysis->>Result: Generate report
    Result->>User: Detailed analysis report
```

### Incident Workflow Data Flow
```mermaid
sequenceDiagram
    participant User
    participant Categorizer
    participant IncidentNode
    participant Prometheus
    participant Loki
    participant Alertmanager
    participant RAG
    participant Analysis
    participant Result
    
    User->>Categorizer: "Investigate payment failures"
    Categorizer->>IncidentNode: Route to Incident workflow
    IncidentNode->>Prometheus: Collect metrics
    IncidentNode->>Loki: Collect logs
    IncidentNode->>Alertmanager: Check alerts
    IncidentNode->>RAG: Search documentation
    
    Prometheus->>Analysis: Metric data
    Loki->>Analysis: Log data
    Alertmanager->>Analysis: Alert data
    RAG->>Analysis: Context docs
    
    Analysis->>Result: Comprehensive investigation
    Result->>User: Root cause + recommendations
```

### Workflow State Transitions
```mermaid
stateDiagram-v2
    [*] --> INITIALIZATION
    INITIALIZATION --> VALIDATION
    VALIDATION --> CATEGORIZATION
    CATEGORIZATION --> DATA_COLLECTION
    DATA_COLLECTION --> DATA_COLLECTION: More data sources
    DATA_COLLECTION --> INCIDENT_INVESTIGATION: Incident workflow
    INCIDENT_INVESTIGATION --> ANALYSIS
    DATA_COLLECTION --> ANALYSIS
    ANALYSIS --> COMPLETED
    COMPLETED --> [*]
    
    VALIDATION --> ERROR: Validation failed
    CATEGORIZATION --> ERROR: Categorization failed
    DATA_COLLECTION --> ERROR: Collection failed
    ANALYSIS --> ERROR: Analysis failed
    ERROR --> [*]
```

## Memory System

### Memory System Architecture
```mermaid
graph TB
    subgraph "Memory System Architecture"
        API[Memory API Layer]
        SERVICE[Memory Service]
        EXTRACTOR[Memory Extractor]
        
        subgraph "Storage Backends"
            QDRANT[(Qdrant<br/>Vector Store)]
            NEO4J[(Neo4j<br/>Graph Store)]
            MEM0[Mem0 Framework]
        end
        
        subgraph "AI Components"
            OPENAI[OpenAI<br/>Embeddings & Analysis]
            EXTRACTOR_AI[AI Memory Extraction]
        end
        
        subgraph "Integration Points"
            WORKFLOW[LangGraph Workflows]
            RAG[RAG System]
            PROMPTS[Intelligent Prompts]
        end
    end
    
    API --> SERVICE
    SERVICE --> MEM0
    SERVICE --> EXTRACTOR
    
    MEM0 --> QDRANT
    MEM0 --> NEO4J
    
    EXTRACTOR --> OPENAI
    SERVICE --> OPENAI
    
    WORKFLOW --> SERVICE
    RAG --> QDRANT
    PROMPTS --> SERVICE
```

### Memory-Enhanced Workflows
```mermaid
graph TD
    A[Workflow Start] --> B[Load Contextual Memories]
    B --> C[Execute Workflow Logic]
    C --> D[Collect Results]
    D --> E{Should Extract Memories?}
    E -->|Yes| F[Extract and Store Memories]
    E -->|No| G[Complete Workflow]
    F --> G
    
    subgraph "Memory Operations"
        B --> H[Search Vector Store]
        B --> I[Query Graph Relationships]
        F --> J[AI-Powered Extraction]
        F --> K[Store in Vector DB]
        F --> L[Create Graph Relationships]
    end
```

## RAG System

### RAG Architecture
```mermaid
graph TB
    subgraph "Document Input"
        UPLOAD[File Upload API]
        PDF[PDF Documents]
        MD[Markdown Documents]
    end
    
    subgraph "Processing Pipeline"
        PARSER[Document Parser]
        CHUNKER[Text Chunker]
        EMBEDDER[Embedding Generator]
    end
    
    subgraph "Storage Layer"
        QDRANT[(Qdrant Vector DB)]
        METADATA[(Metadata Store)]
    end
    
    subgraph "Retrieval Layer"
        SEARCH[Semantic Search]
        FILTER[Metadata Filtering]
        RANK[Result Ranking]
    end
    
    subgraph "Integration Layer"
        WORKFLOW[Alert Workflows]
        API[REST API]
        FRONTEND[Web Interface]
    end
    
    UPLOAD --> PARSER
    PDF --> PARSER
    MD --> PARSER
    
    PARSER --> CHUNKER
    CHUNKER --> EMBEDDER
    EMBEDDER --> QDRANT
    EMBEDDER --> METADATA
    
    SEARCH --> QDRANT
    FILTER --> METADATA
    RANK --> SEARCH
    
    WORKFLOW --> SEARCH
    API --> SEARCH
    FRONTEND --> API
```

## Monitoring Integration

### Monitoring Stack Architecture
```mermaid
graph TB
    subgraph "Paladin AI Core"
        WORKFLOW[LangGraph Workflows]
        MCP[MCP Tool Registry]
        REDUCER[Data Reducer]
        AI[OpenAI GPT-4]
    end
    
    subgraph "Monitoring Stack"
        PROM[Prometheus<br/>Metrics]
        LOKI[Loki<br/>Logs]
        GRAFANA[Grafana<br/>Dashboards]
        ALERT[Alertmanager<br/>Alerts]
    end
    
    subgraph "Tool Services"
        PROM_TOOL[Prometheus Service]
        LOKI_TOOL[Loki Service]
        ALERT_TOOL[Alertmanager Service]
    end
    
    WORKFLOW --> MCP
    MCP --> PROM_TOOL
    MCP --> LOKI_TOOL
    MCP --> ALERT_TOOL
    
    PROM_TOOL --> PROM
    LOKI_TOOL --> LOKI
    ALERT_TOOL --> ALERT
    
    PROM_TOOL --> REDUCER
    LOKI_TOOL --> REDUCER
    ALERT_TOOL --> REDUCER
    
    REDUCER --> AI
    AI --> WORKFLOW
    
    GRAFANA -.-> PROM
    GRAFANA -.-> LOKI
```

### Data Reduction Flow
```mermaid
graph LR
    A[Raw Monitoring Data] --> B[Data Reducer]
    B --> C{Token Limit Check}
    C -->|Under Limit| D[Pass Through]
    C -->|Over Limit| E[Apply Reduction]
    E --> F[Time Filtering]
    F --> G[Aggregation]
    G --> H[Sampling]
    H --> I[Optimized Data]
    D --> J[AI Analysis]
    I --> J
```

### MCP Tools Architecture
```mermaid
graph TB
    subgraph "MCP Tools Architecture"
        WORKFLOW[Workflow Engine]
        MCP_ROUTER[MCP Router]
        
        subgraph "Tool Implementations"
            PROM_TOOL[Prometheus Tool]
            LOKI_TOOL[Loki Tool]
            GRAFANA_TOOL[Grafana Tool]
            ALERT_TOOL[Alertmanager Tool]
        end
        
        subgraph "External Services"
            PROM[Prometheus]
            LOKI[Loki]
            GRAFANA[Grafana]
            ALERT[Alertmanager]
        end
        
        WORKFLOW --> MCP_ROUTER
        MCP_ROUTER --> PROM_TOOL
        MCP_ROUTER --> LOKI_TOOL
        MCP_ROUTER --> GRAFANA_TOOL
        MCP_ROUTER --> ALERT_TOOL
        
        PROM_TOOL --> PROM
        LOKI_TOOL --> LOKI
        GRAFANA_TOOL --> GRAFANA
        ALERT_TOOL --> ALERT
    end
```

### Workflow Execution Flow
```mermaid
sequenceDiagram
    participant User
    participant Workflow
    participant ToolRegistry
    participant MonitoringTools
    participant DataReducer
    participant AI
    
    User->>Workflow: Submit query
    Workflow->>ToolRegistry: Select appropriate tools
    ToolRegistry->>MonitoringTools: Execute data collection
    MonitoringTools->>DataReducer: Process raw data
    DataReducer->>AI: Provide optimized data
    AI->>Workflow: Generate analysis
    Workflow->>User: Return results
```

## Frontend Architecture

### Frontend Data Flow
```mermaid
graph TD
    A[User Input] --> B[Chat Input Component]
    B --> C{Command or Message?}
    C -->|Command| D[Command Handler]
    C -->|Message| E[Message Handler]
    D --> F[API Call]
    E --> F
    F --> G[Response Processing]
    G --> H[State Update]
    H --> I[UI Re-render]
    I --> J[User Sees Result]
```

## Data Flows

### User Interaction Flow
```mermaid
sequenceDiagram
    participant User
    participant Frontend
    participant API Gateway
    participant Paladin Server
    participant Workflow Engine
    participant AI Services
    participant Data Layer
    
    User->>Frontend: Send message
    Frontend->>API Gateway: POST /api/v1/chat
    API Gateway->>Paladin Server: Route request
    Paladin Server->>Workflow Engine: Create workflow
    Workflow Engine->>AI Services: Process with LLM
    AI Services->>Data Layer: Store/retrieve context
    Data Layer-->>AI Services: Return context
    AI Services-->>Workflow Engine: Return response
    Workflow Engine-->>Paladin Server: Complete workflow
    Paladin Server-->>API Gateway: Return response
    API Gateway-->>Frontend: JSON response
    Frontend-->>User: Display response
```

### Document Processing Flow
```mermaid
sequenceDiagram
    participant User
    participant Frontend
    participant RAG Service
    participant Vector DB
    participant LLM
    
    User->>Frontend: Upload document
    Frontend->>RAG Service: POST /api/v1/documents/rag
    RAG Service->>RAG Service: Extract text
    RAG Service->>RAG Service: Chunk document
    RAG Service->>LLM: Generate embeddings
    LLM-->>RAG Service: Return embeddings
    RAG Service->>Vector DB: Store embeddings
    Vector DB-->>RAG Service: Confirm storage
    RAG Service-->>Frontend: Upload complete
    Frontend-->>User: Show success
```

### Monitoring Query Flow
```mermaid
sequenceDiagram
    participant User
    participant Workflow Engine
    participant MCP Tools
    participant Monitoring Stack
    participant Data Reducer
    participant LLM
    
    User->>Workflow Engine: "Check CPU usage"
    Workflow Engine->>MCP Tools: Select Prometheus tool
    MCP Tools->>Monitoring Stack: Query metrics
    Monitoring Stack-->>MCP Tools: Return raw data
    MCP Tools->>Data Reducer: Reduce data size
    Data Reducer-->>MCP Tools: Return reduced data
    MCP Tools-->>Workflow Engine: Formatted data
    Workflow Engine->>LLM: Analyze data
    LLM-->>Workflow Engine: Analysis response
    Workflow Engine-->>User: Human-readable result
```

## Security Architecture

### Authentication & Authorization
```mermaid
graph TB
    subgraph "Authentication Layer"
        AUTH[Authentication Service]
        JWT[JWT Token Manager]
        RBAC[Role-Based Access Control]
    end
    
    subgraph "API Security"
        RATE[Rate Limiting]
        CORS[CORS Protection]
        VALID[Input Validation]
        SANITIZE[Data Sanitization]
    end
    
    subgraph "Data Security"
        ENCRYPT[Encryption at Rest]
        TLS[TLS in Transit]
        SECRETS[Secret Management]
    end
    
    AUTH --> JWT
    JWT --> RBAC
    RBAC --> RATE
    RATE --> CORS
    CORS --> VALID
    VALID --> SANITIZE
    
    ENCRYPT --> TLS
    TLS --> SECRETS
```

## Performance & Scalability

### Horizontal Scaling
```mermaid
graph TB
    subgraph "Horizontal Scaling"
        LB[Load Balancer]
        APP1[App Instance 1]
        APP2[App Instance 2]
        APP3[App Instance 3]
    end
    
    subgraph "Caching Strategy"
        CACHE[Valkey Cache]
        CDN[CDN Layer]
        BROWSER[Browser Cache]
    end
    
    subgraph "Database Scaling"
        MONGO_PRIMARY[MongoDB Primary]
        MONGO_SECONDARY[MongoDB Secondary]
        VECTOR_CLUSTER[Qdrant Cluster]
    end
    
    LB --> APP1
    LB --> APP2
    LB --> APP3
    
    APP1 --> CACHE
    APP2 --> CACHE
    APP3 --> CACHE
    
    CACHE --> MONGO_PRIMARY
    MONGO_PRIMARY --> MONGO_SECONDARY
    CACHE --> VECTOR_CLUSTER
```

## Observability

### System Monitoring
```mermaid
graph TB
    subgraph "Application Metrics"
        APP_METRICS[Application Metrics]
        CUSTOM_METRICS[Custom Metrics]
        BUSINESS_METRICS[Business Metrics]
    end
    
    subgraph "Infrastructure Metrics"
        SYSTEM_METRICS[System Metrics]
        CONTAINER_METRICS[Container Metrics]
        NETWORK_METRICS[Network Metrics]
    end
    
    subgraph "Logging"
        APP_LOGS[Application Logs]
        ACCESS_LOGS[Access Logs]
        ERROR_LOGS[Error Logs]
    end
    
    subgraph "Tracing"
        DISTRIBUTED_TRACING[Distributed Tracing]
        SPAN_TRACKING[Span Tracking]
        CORRELATION_IDS[Correlation IDs]
    end
    
    APP_METRICS --> PROMETHEUS[Prometheus]
    CUSTOM_METRICS --> PROMETHEUS
    BUSINESS_METRICS --> PROMETHEUS
    
    SYSTEM_METRICS --> PROMETHEUS
    CONTAINER_METRICS --> PROMETHEUS
    NETWORK_METRICS --> PROMETHEUS
    
    APP_LOGS --> LOKI[Loki]
    ACCESS_LOGS --> LOKI
    ERROR_LOGS --> LOKI
    
    DISTRIBUTED_TRACING --> JAEGER[Jaeger]
    SPAN_TRACKING --> JAEGER
    CORRELATION_IDS --> JAEGER
```

---

*This document contains all the architectural diagrams from the Paladin AI documentation. Each diagram provides a visual representation of the system's components, data flows, and interactions to help understand the overall architecture and design decisions.*