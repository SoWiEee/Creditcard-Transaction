import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import pool from '../config/db.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const USER_CSV_PATH = path.join(__dirname, 'user.csv');
const TX_CSV_PATH = path.join(__dirname, 'transaction.csv');

const parseCSV = (filePath) => {
  const content = fs.readFileSync(filePath, 'utf-8');
  const lines = content.trim().split('\n');
  const headers = lines[0].trim().split(',');
  
  return lines.slice(1).map(line => {
    const values = line.trim().split(',');
    const entry = {};
    headers.forEach((header, index) => {
      // 處理 CSV 中的 "NULL" 字串轉為真正的 null
      let value = values[index];
      if (value === 'NULL') value = null;
      entry[header.trim()] = value;
    });
    return entry;
  });
};

const seedDatabase = async () => {
  const client = await pool.connect();
  
  try {
    console.log('🌱 Starting database seeding...');
    await client.query('BEGIN');

    // 1. 清空舊資料 (可選，避免重複 ID 錯誤)
    console.log('🧹 Cleaning old data...');
    await client.query('TRUNCATE TABLE Transactions, Users RESTART IDENTITY CASCADE;');

    // 2. 匯入 Users
    console.log('👤 Seeding Users...');
    const users = parseCSV(USER_CSV_PATH);
    
    for (const user of users) {
      // 注意：CSV 缺少 credit_limit，這裡給予預設值 10000
      const query = `
        INSERT INTO Users (user_id, username, balance, current_points, credit_limit)
        VALUES ($1, $2, $3, $4, $5)
      `;
      await client.query(query, [
        user.user_id, 
        user.username, 
        user.balance, 
        user.current_points, 
        10000.00 // 預設額度
      ]);
    }
    console.log(`✅ Inserted ${users.length} users.`);

    // 3. 匯入 Transactions
    console.log('💳 Seeding Transactions...');
    const transactions = parseCSV(TX_CSV_PATH);
    
    for (const tx of transactions) {
      const query = `
        INSERT INTO Transactions 
        (transaction_id, user_id, amount, status, point_change, source_transaction_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
      `;
      // transaction.csv header: transaction_id, user_id, amount, status, point_change, source_transaction_id, created_at
      await client.query(query, [
        tx.transaction_id,
        tx.user_id,
        tx.amount,
        tx.status,
        tx.point_change,
        tx.source_transaction_id, // 這裡如果是 null 會被正確處理
        tx.created_at
      ]);
    }
    console.log(`✅ Inserted ${transactions.length} transactions.`);

    await client.query('COMMIT');
    console.log('🎉 Seeding completed successfully!');
    
  } catch (error) {
    await client.query('ROLLBACK');
    console.error('❌ Seeding failed:', error);
  } finally {
    client.release();
    pool.end();
  }
};

seedDatabase();