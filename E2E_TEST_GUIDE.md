# End-to-End Testing Guide — Rebuild and Browser Testing

This guide walks through rebuilding the Docker stack and manually testing the tenant/organization (C1) and related features in the browser.

---

## Prerequisites

- **Docker** and **Docker Compose** installed
- **Browser** (Chrome, Firefox, Edge)
- Git repo up to date with C1 changes

---

## 1. Rebuild Containers

From the project root (`FreeRangeNotify/`):

```powershell
# Stop any running containers
docker-compose down

# Rebuild images (no cache — ensures latest code)
docker-compose build --no-cache

# Alternative: rebuild without --no-cache (faster, uses cached layers)
# docker-compose build
```

Build targets:

- **notification-service** and **notification-worker**: Go backend (cmd/server, cmd/worker)
- **ui**: Node.js Vite dev server (React)
- **elasticsearch**: Elasticsearch 8.11
- **redis**: Redis 7

---

## 2. Configure Environment

```powershell
# Ensure .env exists
if (!(Test-Path .env)) { Copy-Item .env.example .env }

# Optional: verify JWT secret and basic settings
# FREERANGE_SECURITY_JWT_SECRET should be set in .env
```

Default `.env.example` is sufficient for local E2E. Backend uses `http://elasticsearch:9200` and `redis:6379` inside the Docker network.

---

## 3. Start Services

```powershell
# Start all services in detached mode
docker-compose up -d

# Watch logs until healthy (Ctrl+C to exit)
docker-compose logs -f
```

Give it 30–60 seconds for Elasticsearch to become ready and the notification-service to pass its health check.

**Verify services:**


| Service     | URL                                                                                  | Check                               |
| ----------- | ------------------------------------------------------------------------------------ | ----------------------------------- |
| Backend API | [http://localhost:8080](http://localhost:8080)                                       | `curl http://localhost:8080/health` |
| UI          | [http://localhost:3000](http://localhost:3000)                                       | Open in browser                     |
| Swagger     | [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) | Open in browser                     |


---

## 4. Browser E2E Test Flow — Tenant/Organization (C1)

### 4.1 Create an Account and Log In

1. Open **[http://localhost:3000](http://localhost:3000)**
2. Click **Register** (or go to `/register`)
3. Fill in **Email**, **Password**, **Full Name**
4. Submit → you should be redirected to the dashboard
5. If already registered, use **Login** instead

### 4.2 Organizations (Tenants)

1. In the sidebar, click **Organizations**
2. You should see the Organizations page (possibly empty)
3. Click **+ New Organization**
4. Enter a name (e.g. `Acme Inc`) → **Create Organization**
5. Click the new organization card → you should land on the detail page
6. On the Members tab:
  - Invite someone: **Invite** → enter email (must exist in the system) and role (Admin/Member) → **Send Invite**
  - Change role: use the role dropdown on a member (owner cannot be changed)
  - Remove: trash icon on non-owner members
7. Use **Back to Organizations** to return to the list

### 4.3 Create App Under an Organization

1. Go to **Applications**
2. Click **+ New Application**
3. If you have organizations, the **Organization (optional)** dropdown appears
4. Select an organization (or keep **Personal workspace**)
5. Enter **Application Name** and **Description** → **Create Application**
6. Confirm the app appears in the list and opens in detail view

### 4.4 Quick Checks

- **Applications**: list, create, open detail
- **Command palette**: `Ctrl+K` (or `Cmd+K` on Mac) → search for "Organizations" → should navigate to `/tenants`
- **Sidebar**: Organizations link under MAIN
- **Swagger**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) — verify `/v1/tenants` endpoints

---

## 5. Troubleshooting

### Containers won't start

```powershell
docker-compose down -v
docker-compose up -d
docker-compose logs -f
```

### Backend 500 or health check fails

- Check Elasticsearch: `curl http://localhost:9200/_cluster/health`
- Inspect logs: `docker-compose logs notification-service`

### UI can't reach API (CORS / proxy)

- UI uses Vite proxy: `/v1` → `http://notification-service:8080`
- Ensure `API_PROXY_TARGET` is not overridden; Docker sets it to `http://notification-service:8080`
- Browser should use relative URLs (e.g. `/v1/auth/login`)

### Invite fails

- Invited user must already be registered (email exists)
- Error text in toast indicates the reason

### Port conflicts

- Backend: 8080
- UI: 3000
- Elasticsearch: 9200, 9300
- Redis: 6379

Change ports via environment if needed:

```powershell
$env:FREERANGE_SERVER_PORT = "8081"
docker-compose up -d
# UI remains on 3000; backend becomes http://localhost:8081
```

---

## 6. Stop Services

```powershell
# Stop containers (preserves volumes)
docker-compose down

# Stop and remove volumes (clean slate)
docker-compose down -v
```

---

## 7. Summary — C1-Related UI Surfaces


| Feature             | Location                         | Action                 |
| ------------------- | -------------------------------- | ---------------------- |
| Organizations list  | Sidebar → Organizations          | Create and view orgs   |
| Organization detail | Click an org card                | Manage members         |
| Invite member       | Org detail → Invite button       | Email + role           |
| Create app with org | Applications → + New Application | Optional org dropdown  |
| Command palette     | Ctrl+K                           | "Organizations" search |


