import pool from '../config/db.js';
import * as UserModel from '../models/userModel.js';
import * as TxModel from '../models/transactionModel.js';

const withTransaction = async (callback) => {
    const client = await pool.connect();
    try {
        await client.query('BEGIN');
        const result = await callback(client);
        await client.query('COMMIT');
        return result;
    } catch (e) {
        await client.query('ROLLBACK');
        throw e;
    } finally {
        client.release();
    }
};

export const getUserDetails = async (userId) => {
    const client = await pool.connect();
    try {
        const user = await UserModel.getUserById(client, userId);

        if (!user) {
            throw new Error('User not found');
        }

        // 可以在這裡過濾掉敏感資訊 (例如密碼 hash)，如果有的話
        return user; 
    } finally {
        client.release();
    }
};

export const getTransactionHistory = async (userId) => {
    const client = await pool.connect();
    try {
        const transactions = await TxModel.getTransactionsByUserId(client, userId);
        return transactions;
    } finally {
        client.release();
    }
};

export const processPayment = async (userId, amount) => {
    return withTransaction(async (client) => {
        // 1. 檢查使用者
        const user = await UserModel.getUserById(client, userId);
        if (!user) throw new Error('User not found');

        // 2. 檢查額度 (Business Rule)
        const newBalance = parseFloat(user.balance) + parseFloat(amount);
        if (newBalance > parseFloat(user.credit_limit)) {
            throw new Error('Insufficient credit limit');
        }

        // 3. 計算點數 (Business Rule: 1%)
        const pointsEarned = Math.floor(amount * 0.01);

        // 4. 生成 ID (模擬)
        const maxId = await TxModel.getMaxTransactionId(client);
        const newTxId = maxId + 1;

        // 5. 寫入交易紀錄
        await TxModel.createTransaction(client, {
            id: newTxId,
            userId,
            amount,
            status: 'Paid', // 或 Pending，視需求
            pointChange: pointsEarned,
            sourceId: null
        });

        // 6. 更新使用者餘額
        await UserModel.updateUserBalanceAndPoints(client, userId, amount, pointsEarned);

        return { transactionId: newTxId, status: 'Paid', points: pointsEarned };
    });
};

export const voidTransaction = async (userId, targetTxId) => {
    return withTransaction(async (client) => {
        // 1. 鎖定並查詢目標交易
        const tx = await TxModel.getTransactionById(client, targetTxId);
        if (!tx) throw new Error('Transaction not found');

        // 2. 狀態檢查
        if (Number(tx.user_id) !== Number(userId)) {
            throw new Error('Security Alert: You do not own this transaction.');
        }
        if (tx.status !== 'Pending') {
            throw new Error(`Cannot void transaction with status: ${tx.status}`);
        }

        // 3. 更新交易狀態
        await TxModel.updateTransactionStatus(client, targetTxId, 'Voided');

        // 4. 恢復餘額 (Void 不涉及點數回扣，因為 Pending 時未給點)
        // 注意：這裡扣除原交易金額，讓 Balance 變小 (恢復額度)
        await UserModel.updateUserBalanceAndPoints(client, userId, -tx.amount, 0);

        return { success: true, message: 'Transaction voided' };
    });
};

export const refundTransaction = async (userId, targetTxId) => {
    return withTransaction(async (client) => {
        // 1. 查詢原交易
        const tx = await TxModel.getTransactionById(client, targetTxId);
        if (!tx) throw new Error('Transaction not found');

        // prevent IDOR
        if (Number(tx.user_id) !== Number(userId)) {
            throw new Error('Security Alert: Unauthorized refund attempt.');
        }

        // 2. 狀態檢查
        if (tx.status !== 'Paid') {
            throw new Error(`Cannot refund transaction with status: ${tx.status}`);
        }

        // 3. 檢查使用者點數是否足夠扣回 (Cascading Check)
        const user = await UserModel.getUserById(client, userId);
        if (user.current_points < tx.point_change) {
            throw new Error('Insufficient points to rollback transaction');
        }

        // 4. 標記原交易
        await TxModel.updateTransactionStatus(client, targetTxId, 'Refunded');

        // 5. 建立補償交易 (負向)
        const maxId = await TxModel.getMaxTransactionId(client);
        const refundTxId = maxId + 1;

        await TxModel.createTransaction(client, {
            id: refundTxId,
            userId,
            amount: -tx.amount,      // 負金額
            status: 'Refunded',
            pointChange: -tx.point_change, // 扣回點數
            sourceId: targetTxId
        });

        // 6. 更新使用者 (扣錢，扣點)
        await UserModel.updateUserBalanceAndPoints(client, userId, -tx.amount, -tx.point_change);

        return { refundTransactionId: refundTxId, success: true };
    });
};