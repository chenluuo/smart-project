# 08. 首次真实联调结果

## 环境

- MySQL: Docker container `smart-agriculture-mysql`, host port `3307`, healthy
- MinIO: Docker container `smart-agriculture-minio`, host ports `9000/9001`, healthy
- Go API: Docker container `smart-agriculture-api`, Go `1.25`, host port `8080`
- Frontend: Vite + React + TypeScript, `http://127.0.0.1:5173/`

## Passed checks

- `GET /actuator/health`: `UP`
- `POST /api/v1/auth/register`: passed
- `POST /api/v1/auth/login`: passed
- `GET /api/v1/users/me`: passed
- `GET /api/v1/plots`: passed, empty list
- `GET /api/v1/dashboard/overview`: passed
- `GET /api/v1/devices`: passed, empty list
- `GET /api/v1/alerts`: passed, empty list
- `GET /api/v1/commands`: passed, empty list
- `GET /api/v1/knowledge/docs`: passed, empty list
- `GET /api/v1/events/stream`: authenticated SSE connection passed; initial `connected` event received
- Vite proxy login through `http://127.0.0.1:5173/api/v1/auth/login`: passed

## Populated data run

- Seeded one owned plot: `A3`
- Seeded one online soil moisture sensor and one online irrigation valve
- Seeded one soil moisture threshold rule and one active alert
- Seeded one active irrigation knowledge document
- Dashboard overview: one plot, two online devices, one active alert
- Device list: two items
- Device status: online, battery `89`, signal `82`
- Irrigation status before control: `OFF`
- Irrigation command: `OPEN`, `600` seconds, `SUCCESS`
- Irrigation status after control: `ON`, `600` seconds remaining
- Command list: one `SUCCEEDED` item
- Vite proxy reads for Dashboard, devices, irrigation, alerts, and knowledge: passed
- Alert confirmation through the Vite proxy: `ACTIVE` -> `CONFIRMED`, remark persisted, Dashboard active alert count changed from `1` to `0`
- Knowledge upload through the Vite proxy: passed with a dedicated `SYSTEM_ADMIN` test account
- Knowledge document workflow: upload `DRAFT` -> approve `APPROVED` -> publish `ACTIVE`, all passed
- Published document list: one active document returned with a signed URL
- Signed download: `200`, `61` bytes, `text/markdown`

## Notes

- The first run used an empty database and confirmed the response contract. The populated run now has a small local test dataset.
- The host machine does not have Go installed. The backend was run from the repository source in an official Go `1.25` Docker container.
- The only code fix in the populated run was changing the irrigation repository query from GORM `First` to `Take`, preventing an invalid implicit `ORDER BY p.device_id`. The API contract was unchanged.
- Telemetry is still backed by `telemetry.NullStore`, so soil moisture and temperature remain `null` until a telemetry store is connected.
- The alert confirmation run changed only local test data; no frontend code was changed.
- For local Docker, `MINIO_ENDPOINT` is the internal API-to-MinIO address and `MINIO_PUBLIC_ENDPOINT` is the browser-facing signing address. For LAN access, the current public address is `192.168.20.165:9000`.

## Next task

Connect a telemetry-backed Dashboard data source and verify soil moisture, temperature, and sample time.
