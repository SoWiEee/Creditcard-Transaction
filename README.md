# Creditcard-Transaction
> 信用卡交易與點數系統開發規格書

# Project Overview

本系統旨在模擬信用卡核心交易流程，包含授權 (Authorization)、結算 (Settlement)、作廢 (Void) 與退款 (Refund)。系統必須嚴格遵守 ACID 原則，確保帳戶餘額與紅利點數在任何交易狀態下的一致性。
- 前端架構: Vue 3 + Vite + Bun (Single Page Application)
- 後端架構: Node.js/Bun (REST API)
- 資料庫: PostgreSQL

# Database Schema

根據提供的 SQL.sql 檔案，資料庫包含兩個核心資料表。

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




