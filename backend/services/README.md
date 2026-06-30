# BidWriter Services

Each service is an independent Go module. See `docs/architecture/modules.md`.

| Service | Port | Purpose |
|---|---|---|
| api-gateway | 8080 | Auth, routing, rate limiting |
| project-svc | 8081 | Project CRUD |
| document-svc | 8082 | Document upload/parse |
| workflow-svc | 8083 | Workflow state machine |
| knowledge-svc | 8084 | Knowledge base + RAG |
| router-svc | 8085 | AI model routing |
| template-svc | 8086 | Templates + marketplace |
| billing-svc | 8087 | Subscriptions + usage |
| notify-svc | 8088 | Email / webhook / IM |
| audit-svc | 8089 | Consistency audit |
