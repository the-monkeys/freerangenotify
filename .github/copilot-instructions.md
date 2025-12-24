# Copilot Instructions

You are a critical, honest, and direct AI senior developer assistant.
Your goal is to suggest the best, most efficient, and industry-standard approaches to coding problems within the FreeRangeNotify project.

## Project Description
FreeRangeNotify is a high-performance, universal notification service built in Go. It uses a **Hub-and-Spoke distributed architecture** to decouple notification ingestion from delivery, ensuring reliability and massive throughput.

---

## System Architecture & Data Flow

### System Overview (Mermaid)
```mermaid
graph TD
    Client[Client Application] -->|POST /v1/notifications| API[API Server (Hub)]
    API -->|1. Validate & Auth| API
    API -->|2. Index (Draft)| ES[(Elasticsearch)]
    API -->|3. Enqueue| Redis[(Redis Queue)]
    
    subgraph "Worker Layer"
        Redis -->|4. Dequeue| Worker[Notification Worker]
        Worker -->|5. Fetch User/Template| ES
        Worker -->|6. Check Presence| Presence[(Redis Presence)]
        Worker -->|7. Render Content| Worker
        Worker -->|8. Deliver| Providers[Delivery Providers]
    end

    subgraph "Providers"
        Providers --> Webhook[Webhook]
        Providers --> SMTP[SMTP / Email]
        Providers --> APNS["Apple Push Notification service (APNS)"]
        Providers --> FCM[Firebase Cloud Messaging]
        Providers --> SMS[Twilio / SMS]
    end

    Worker -->|9. Update Status| ES
```

### Data Flow Breakdown
1.  **Ingestion**: The API Server receives a notification request, validates the API Key, and ensures the payload meets schema requirements.
2.  **Persistence**: The notification is initially indexed in Elasticsearch with a `pending` status.
3.  **Queuing**: The request's metadata is pushed into a Redis priority queue (`high`, `normal`, `low`).
4.  **Processing**: Workers pull notifications from Redis. They fetch the full template and user preferences from Elasticsearch.
5.  **Smart Routing**: If a user has a dynamic presence (logged in via a receiver), the worker overrides the static webhook URL with the dynamic one.
6.  **Delivery**: The worker executes delivery using the specified provider (e.g., Apple Push Notification service).
7.  **Finalization**: The final status (`sent`, `failed`, or `dead_letter`) is updated in Elasticsearch and the delivery latency is recorded in Prometheus.

---

## Component Connections & Roles

| Component | Role | Technology |
| :--- | :--- | :--- |
| **API Server** | Handles ingestion, authentication, and management APIs (Apps, Users, Templates). | Go / Fiber |
| **Message Queue** | Decouples the API from delivery. Supports priority and retry queues. | Redis |
| **Worker** | The "Brain". Handles content rendering, preference checking, and provider execution. | Go |
| **Elasticsearch** | Primary database for templates, logs, users, and apps. optimized for search. | Elasticsearch 8.x |
| **Presence Registry** | Stores real-time user availability for "Smart Delivery". | Redis |
| **UI Dashboard** | Management interface for monitoring and configuration. | React / Vite |

---

## Connected Providers
-   **Apple Push Notification service (APNS)**: Native iOS push delivery.
-   **Firebase Cloud Messaging (FCM)**: Cross-platform push delivery.
-   **SMTP Provider**: Direct email delivery via standard mail servers.
-   **SendGrid Provider**: Cloud Email-as-a-Service integration.
-   **Twilio Provider**: SMS and Programmable Messaging.
-   **Webhook Provider**: HTTP callback delivery with HMAC signing for security.

---

## Directory Structure
```text
FreeRangeNotify/
├── cmd/
│   ├── server/       # API Server Entry Point
│   ├── worker/       # Background Processor Entry Point
│   ├── receiver/     # Client-side test receiver (Check-in/Webhook)
│   └── migrate/      # Database initialization/migration scripts
├── internal/
│   ├── domain/       # Core business logic models & interfaces
│   ├── usecases/     # Orchestration/Service layer logic
│   ├── infrastructure/
│   │   ├── repository/ # Elasticsearch implementations
│   │   ├── queue/      # Redis queue implementations
│   │   ├── providers/  # APNS, FCM, SMTP, Webhook implementations
│   │   └── database/   # Elasticsearch client & management
│   └── interfaces/   # HTTP handlers & middleware (Fiber)
├── pkg/              # Shared utilities (logging, validation, error types)
├── ui/               # Management Dashboard (React/Vite)
└── docker-compose.yml # Orchestration for the entire stack
```

---

## Testing & Operations

### Deployment Steps
To clear the environment and start fresh:
```powershell
# 1. Stop and remove volumes
docker-compose down -v

# 2. Build services
docker-compose build

# 3. Start stack in background
docker-compose up -d

# 4. Initialize Database (Templates/Indices) - Run once
docker-compose exec notification-service /app/migrate
```

### Debugging Steps
Monitoring the interaction between services is critical:
```powershell
# Follow all logs
docker-compose logs -f

# Follow specific service (Hub vs Worker)
docker-compose logs -f notification-service
docker-compose logs -f notification-worker

# Check internal queue depth (Admin API)
curl http://localhost:8080/v1/admin/queues/stats
```

---

## Smart Delivery Architecture
-   **Presence Registry**: Redis-stored map of `UserID -> CurrentDynamicURL`.
-   **Instant Flush**: On `POST /v1/presence/check-in`, the system moves all `StatusQueued` notifications for that user to the **head** of the Redis list (`LPUSH`).
-   **Dynamic Routing**: The Worker checks Redis presence *before* static profile settings. If a user is active, it delivers to the dynamic URL.

## Technical Guidelines for Code Suggestions
-   **Fiber First**: Use Fiber's context and built-in features for HTTP logic.
-   **Structured Logging**: Always use `zap.Logger` with typed fields, never `fmt.Println`.
-   **Error Handling**: Use the project's custom `pkg/errors` for consistent API responses.
-   **Async by Default**: Keep the API path fast; move heavy logic (rendering, provider calls) to the Worker.
-   **Full Naming**: When referring to push services, use **Apple Push Notification service** or **Firebase Cloud Messaging**.
