# Creditcard-Transaction

- [Test Website](https://dbdev.mintice.blog/)

> 信用卡交易與點數系統開發規格書

# Project Overview

本系統旨在模擬信用卡核心交易流程，包含授權 (Authorization)、結算 (Settlement)、作廢 (Void) 與退款 (Refund)。系統必須嚴格遵守 ACID 原則，確保帳戶餘額與紅利點數在任何交易狀態下的一致性。
- 前端架構：Vue 3 + Vite + Bun (Single Page Application)
- 後端架構：Node.js/Bun (REST API)
- 資料庫：PostgreSQL

# Database Schema

## Users (使用者)

| 欄位名稱 | 資料型別      | 限制         | 說明                    |
| -------- | ------------- | ------------ | ----------------------- |
| user_id  | INT           | PRIMARY KEY  | 使用者唯一 ID           |
| name     | VARCHAR(100)  | NOT NULL     | 使用者姓名              |
| balance  | DECIMAL(10,2) | DEFAULT 0.00 | 當前消費總額 (負債金額) |
| credit_limit  |  DECIMAL(10,2)   |  NOT NULL      |  信用額度上限             |
| current_points  |  INT    | DEFAULT 0   | 目前持有的紅利點數     |

## Transactions (交易紀錄)

- 記錄所有交易流水帳，包含消費、作廢與退款紀錄。

| 欄位名稱 | 資料型別 | 限制  | 說明 |
| -------- | -------- | --- | -------- |
| transaction_id  |  INT   | PRIMARY KEY  | 交易流水號   |
| user_id  |  INT  |  REFERENCES Users(user_id)  | 關聯使用者 ID  |
| amount  | DECIMAL(10,2)   |  NOT NULL | 交易金額 (正數=消費, 負數=退款)   |
| transaction_date  | TIMESTAMP  |  DEFAULT CURRENT_TIMESTAMP  | 交易時間   |
| status  |  VARCHAR(20)   | CHECK (...)  |  狀態: 'Pending', 'Paid', 'Voided', 'Refunded'   |
| point_change  |  INT    |  DEFAULT 0  | 該筆交易產生的點數變化   |
| source_transaction_id   | INT   | NULLABLE  | 僅用於 Refund，記錄原始交易 ID   |

# API Specification
- 所有 API 回應皆為 JSON 格式。 Base URL: /api

## 取得使用者資訊

- 用於前端初始化 Dashboard，顯示餘額與點數
- GET /users/:id
- Request Parameters: id: 使用者 ID (Int)

Response (200 OK)
```json
{
  "user_id": 1,
  "name": "Alice",
  "balance": 500.00,
  "credit_limit": 10000.00,
  "current_points": 150
}
```

## 取得交易列表

- 用於顯示交易歷史紀錄表格。
- GET /transactions/:user_id

Response (200 OK)
```json
[
    {
      "transaction_id": 105,
      "amount": 300.00,
      "transaction_date": "2023-10-27T10:00:00Z",
      "status": "Paid",
      "point_change": 3,
      "source_transaction_id": null
    },
    {
      "transaction_id": 106,
      "amount": -5000.00,
      "transaction_date": "2023-10-28T12:00:00Z",
      "status": "Refunded",
      "point_change": -50,
      "source_transaction_id": 102
    }
]
```

## 新增交易

- 一般消費功能，需檢查信用額度並計算回饋點數。
- POST /transactions/pay
- Request Body:

```json
{
  "user_id": 4,
  "amount": 300.00
}
```

Response (201 Created)
```json
{
  "success": true,
  "transaction_id": 105,
  "message": "Payment successful",
  "new_balance": 800.00,
  "points_earned": 3
}
```

Error Response (400 Bad Request)
```json
{ "success": false, "error": "Insufficient credit limit" }
```

## 取消交易

- 針對尚未請款 (Pending) 的交易進行作廢。
- POST /transactions/void
- Request Body:

```json
{
  "user_id": 2,
  "target_transaction_id": 103
}
```

Response (200 OK)
```json
{
  "success": true,
  "message": "Transaction 103 voided successfully"
}
```  

## 退款交易

- 針對已請款 (Paid) 的交易進行退貨，需執行「補償交易」(Compensating Transaction) 並扣回點數。
- POST /transactions/refund
- Request Body:

```json
{
  "user_id": 3,
  "target_transaction_id": 102
}
```

Response (200 OK)
```json
{
  "success": true,
  "new_transaction_id": 106,
  "message": "Refund processed successfully"
}
```

Error Response (409 Conflict)
```json
{ "success": false, "error": "Insufficient points for refund rollback" }
```
