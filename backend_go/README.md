# Backend (Go 1.24) - Credit Card / Points Demo API

這個專案是一個簡化版的「信用額度 + 點數 + 交易」後端 API，提供：

- 查詢使用者資訊 / 交易紀錄
- 付款（可選擇用點數折抵）
- 作廢（Void）/ 退款（Refund）
- 內建風險控管（Redis 速度限制 + DB 重複交易 / 退款濫用檢查）

> 設計目標：用清楚的分層把 **HTTP / 商業邏輯 / DB 存取** 分開，方便擴充與測試。

---

## 目錄結構與每個 package 的職責

> 你要求的切分方式：`controller`, `initialize`, `middlewares`, `models`, `repo`, `routers`, `service`, `utils` 都在**同一層**。

- `cmd/server/`：HTTP 入口點（main）
- `cmd/seed/`：DB seed 工具（讀取 `db/seed/*.csv`）
- `controller/`：HTTP handlers（解析 request / 回傳 response）
- `routers/`：集中定義路由（URL -> handler）
- `middlewares/`：跨切面（CORS 等）
- `service/`：商業邏輯（付款/作廢/退款 + 風控）
- `repo/`：資料存取層（SQL 查詢、Row mapping、Tx 操作）
- `models/`：純資料結構（User / Transaction）
- `initialize/`：初始化與組裝（env、DB、Redis、依賴 wiring）
- `utils/`：共用工具（JSON、TxLogger）

---

## 需求與依賴

- Go 1.24（此專案 `Dockerfile` 以 `golang:1.24-alpine` 為 build image）
- PostgreSQL（Tables: `Users`, `Transactions`, `Points`）
- Redis（風控：速度限制 key / window）

---

## 快速開始

### 1) 準備環境變數

你可以用 `.env`（範例已附），或手動 export。

必要參數（擇一）：
- **DB**：`DATABASE_URL`（推薦）
- **Redis**：`REDIS_ADDR=host:port`（推薦）

### 2) 啟動 DB / Redis（示例：使用 docker）

```bash
# Postgres

docker run --rm --name pg \
  -e POSTGRES_USER=cct_user \
  -e POSTGRES_PASSWORD=cct_pass \
  -e POSTGRES_DB=creditcard \
  -p 5432:5432 postgres:16

# Redis

docker run --rm --name rds -p 6379:6379 redis:7
```

### 3) 建表（如果你沒有既有 schema）

> 專案內沒有附 migration 檔，以下提供最小可運作 schema。

```sql
CREATE TABLE IF NOT EXISTS Users (
  user_id INT PRIMARY KEY,
  username VARCHAR(50) NOT NULL,
  balance NUMERIC(10,2) DEFAULT 0.00,
  current_points INT DEFAULT 0,
  credit_limit NUMERIC(10,2) DEFAULT 10000.00
);

CREATE TABLE IF NOT EXISTS Transactions (
  transaction_id SERIAL PRIMARY KEY,
  user_id INT NOT NULL,
  amount NUMERIC(10,2) NOT NULL,
  status VARCHAR(20) CHECK (status IN ('Pending','Paid','Voided','Refunded')) NOT NULL,
  merchant VARCHAR(50),
  point_change INT DEFAULT 0,
  source_transaction_id INT DEFAULT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES Users(user_id),
  FOREIGN KEY (source_transaction_id) REFERENCES Transactions(transaction_id)
);

CREATE TABLE IF NOT EXISTS Points (
  log_id SERIAL PRIMARY KEY,
  user_id INT NOT NULL,
  transaction_id INT,
  change_amount INT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES Users(user_id),
  FOREIGN KEY (transaction_id) REFERENCES Transactions(transaction_id)
);
```

### 4) Seed（可選）

`cmd/seed` 會讀取 `db/seed/user.csv` 與 `db/seed/transaction.csv`，並清空舊資料後重新匯入。

```bash
# 需要先設定 DATABASE_URL

go run ./cmd/seed
```

### 5) 啟動 API server

```bash
go run ./cmd/server
# 預設 PORT=3000
```

---

## API 介面

Base path：`/api`

| Method | Path | 說明 |
|---|---|---|
| GET  | `/api/health` | Health check |
| GET  | `/api/users/{id}` | 查詢使用者資訊 |
| GET  | `/api/transactions/{user_id}` | 查詢該使用者交易紀錄（新到舊） |
| POST | `/api/transactions/pay` | 付款（可選擇用點數折抵） |
| POST | `/api/transactions/void` | 作廢（void）一筆交易 |
| POST | `/api/transactions/refund` | 退款（refund）一筆交易 |

### POST `/api/transactions/pay`

Request:
```json
{
  "user_id": 1,
  "amount": 120.5,
  "merchant": "Steam",
  "use_points": true
}
```

Response (201):
```json
{
  "transactionId": 123,
  "finalAmount": 119.50,
  "pointsEarned": 239,
  "pointsRedeemed": 100,
  "logs": ["..."]
}
```

### POST `/api/transactions/void`

Request:
```json
{
  "user_id": 1,
  "target_transaction_id": 123
}
```

Response (200):
```json
{
  "success": true,
  "voidedAmount": 119.50,
  "restoredPoints": -139,
  "logs": ["..."]
}
```

### POST `/api/transactions/refund`

Request:
```json
{
  "user_id": 1,
  "target_transaction_id": 123
}
```

Response (200):
```json
{
  "refundTransactionId": 124,
  "logs": ["..."]
}
```

---

## 資料流（Data Flow）

### 共通 pipeline

```
Client
  │ HTTP
  ▼
routers/ (chi routes)
  │ r.Use(middlewares.CORS())
  ▼
controller/ (handlers: parse/validate)
  │ call
  ▼
service/ (business rules + risk + tx wrapper)
  │ call
  ▼
repo/ (SQL / persistence)
  │
  ├── Postgres (Users, Transactions, Points)
  └── Redis (風控 velocity)
  ▼
Response (JSON)
```

### 付款（PAY）資料流

入口：`POST /api/transactions/pay`

```
controller.Pay
  └─ service.TransactionService.ProcessPayment
       └─ withTransaction()  // BEGIN/COMMIT/ROLLBACK + TxLogger
            ├─ RiskEngine.EvaluatePaymentRisk
            │    ├─ 金額上下限檢查（Min/Max）
            │    ├─ Redis velocity：INCR + EXPIRE
            │    ├─ DB refund 濫用：24h 內 Refunded 筆數
            │    └─ DB duplicate：同 merchant/amount 在短時間內是否出現
            ├─ SELECT Users ... FOR UPDATE（鎖住使用者）
            ├─ 點數折抵：100 pts = $1（最多折到整數美元且不超過 amount）
            ├─ 信用額度檢查：balance + finalAmount <= credit_limit
            ├─ INSERT Transactions ... RETURNING transaction_id
            ├─ INSERT Points（Redeemed / Earned，可選）
            └─ UPDATE Users (balance += finalAmount, current_points += netPointChange)
```

回傳：成功會回 `transactionId/finalAmount/pointsEarned/pointsRedeemed`，並帶 `logs`（TxLogger 內容）。

### 作廢（VOID）資料流

入口：`POST /api/transactions/void`

```
controller.VoidTx
  └─ service.TransactionService.VoidTransaction
       └─ withTransaction()
            ├─ SELECT Transactions ... FOR UPDATE
            ├─ 權限檢查：交易 user_id 必須等於 request.user_id
            ├─ 狀態檢查：不可 void 已 Voided / Refunded 的交易
            ├─ UPDATE Transactions SET status='Voided'
            ├─ UPDATE Users SET balance = balance + amount
            └─ UPDATE Users current_points 反向回滾 + INSERT Points (Void Reversal)
```

### 退款（REFUND）資料流

入口：`POST /api/transactions/refund`

```
controller.RefundTx
  └─ service.TransactionService.RefundTransaction
       └─ withTransaction()
            ├─ SELECT Transactions ... FOR UPDATE (target)
            ├─ 權限檢查：交易 user_id 必須等於 request.user_id
            ├─ 狀態檢查：僅允許退款 status='Paid' 的交易
            ├─ 讀取使用者目前點數，確保足夠回滾（CurrentPoints >= target.PointChange）
            ├─ UPDATE target transaction status => 'Refunded'
            ├─ INSERT 一筆新的 Transactions（amount 與 point_change 取負值；source_transaction_id 指向原交易）
            ├─ UPDATE Users (balance += refundAmount, current_points += refundPoints)
            └─ INSERT Points (Refund)
```

---

## 各層更細的責任邊界（建議規範）

### controller/
- 解析：URL params / JSON body
- 基本驗證：型別、是否為正數、必要欄位是否存在
- 轉換錯誤：把 `service.TxError` 轉成 HTTP status + 統一錯誤格式

### routers/
- 僅負責 URL 與 handler 映射
- router-level middleware 在這裡掛載（例如 CORS）

### middlewares/
- 跨切面處理：CORS、Auth、Rate limit、Request ID、Logging 等

### service/
- 所有領域規則都應放在這層：
  - 付款/作廢/退款的規則
  - 風控（RiskEngine）
  - 使用 `withTransaction()` 封裝 BEGIN/COMMIT/ROLLBACK 並蒐集 TxLogger logs
- **不處理 HTTP**（不讀 request，不寫 response）

### repo/
- 所有 SQL 存取與 mapping 都在這層
- 函數簽名接受 `repo.Querier`，因此可用於：
  - `*pgxpool.Pool`（自動在池上跑 query）
  - `pgx.Tx`（交易內 query）

### models/
- 只有資料結構（struct），避免放 business logic

### initialize/
- 讀取 env、建立 DB pool / Redis client、組裝（wiring）App

### utils/
- 共用工具：
  - `WriteJSON/ReadJSON`
  - `TxLogger`（把 SQL/Info logs 帶回給前端或 debug 用）

---

## 設定（Environment Variables）

### Server / DB / Redis

| 變數 | 說明 | 預設 |
|---|---|---|
| `PORT` | HTTP listen port | `3000` |
| `DATABASE_URL` | Postgres 連線字串（推薦） | (無) |
| `DB_HOST` `DB_PORT` `DB_USER` `DB_PASSWORD` `DB_NAME` | 若沒有 `DATABASE_URL`，則用這組組成連線字串 | 見 `.env` / code |
| `REDIS_ADDR` | Redis 位址 `host:port`（推薦） | `redis:6379` |
| `REDIS_HOST` `REDIS_PORT` | 若沒有 `REDIS_ADDR`，則用這組組成位址 | `redis` / `6379` |
| `REDIS_PASSWORD` | Redis password | (空) |
| `REDIS_DB` | Redis DB index | `0` |
| `LOADTEST` | `true` 時放寬風控規則 | `false` |

### Docker entrypoint（可選）

| 變數 | 說明 | 預設 |
|---|---|---|
| `RUN_SEED` | container 啟動時先跑 `/app/seed` | `0` |
| `WAIT_FOR_DEPS` | 啟動前等待 DB/Redis 可連線 | `1` |
| `DB_WAIT_TIMEOUT` | 等待 Postgres 秒數 | `30` |
| `REDIS_WAIT_TIMEOUT` | 等待 Redis 秒數 | `10` |

---

## 風控規則（RiskEngine）摘要

風控位於 `service/risk.go`，預設（`LOADTEST=false`）

- 金額限制：`1 <= amount <= 10000`
- 速度限制：同一 user 在 `60s` 內最多 `3` 筆（Redis `INCR + EXPIRE`）
- 重複交易：同 user、同 merchant、同 amount，在 `5m` 內若已出現，判定可能重複
- 退款濫用：24h 內 `Refunded` 筆數達 `3`，暫時凍結（拒絕付款）

---

## 錯誤回傳格式

所有錯誤都用 JSON，格式：

```json
{
  "code": "SOME_CODE",
  "error": "Human readable message",
  "logs": ["...optional tx logs..."]
}
```

其中 `logs` 是 `TxLogger` 產生的 debug 訊息（包含模擬 SQL logs / info logs），方便你在前端或測試時顯示「資料流/執行軌跡」。

---

## Docker（build / run）

```bash
# build

docker build -t backend_go .

# run（請確保你有提供 DATABASE_URL + Redis 設定）

docker run --rm -p 3000:3000 \
  -e PORT=3000 \
  -e DATABASE_URL='postgres://cct_user:cct_pass@host.docker.internal:5432/creditcard' \
  -e REDIS_ADDR='host.docker.internal:6379' \
  backend_go
```

如果你想 container 起來就 seed：

```bash
docker run --rm -p 3000:3000 \
  -e RUN_SEED=1 \
  -e DATABASE_URL='...' \
  -e REDIS_ADDR='...' \
  backend_go
```

---

## 擴充指南（新增一支 API 的最小步驟）

1) 在 `service/` 加入或擴充一個方法（商業規則）
2)（需要 DB 時）在 `repo/` 加入對應的 query function
3) 在 `controller/handlers.go` 加 handler（解析參數 / 呼叫 service）
4) 在 `routers/router.go` 掛上路由

建議：
- handler 保持「薄」，盡量把可變規則放在 service。
- SQL 統一放在 repo，service 不要直接寫長 SQL（除了非常小的臨時 query）。
