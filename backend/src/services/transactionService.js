import pool from '../config/db.js';
import * as UserModel from '../models/userModel.js';
import * as TxModel from '../models/transactionModel.js';

const withTransaction = async (callback) => {
    const client = await pool.connect();
    const logs = [];

    // 內部用的 logger
    const logger = {
        info: (msg) => logs.push(`[INFO] ${msg}`),
        sql: (msg) => logs.push(`[SQL] ${msg}`),
        raw: (msg) => logs.push(msg)
    };

    try {
        logger.sql('START TRANSACTION;');
        await client.query('BEGIN');

        const result = await callback(client, logger); // 將 logger 傳給業務邏輯

        logger.sql('COMMIT;');
        await client.query('COMMIT');

        return { ...result, logs }; // 回傳結果 + Logs
    } catch (e) {
        await client.query('ROLLBACK');
        logger.sql('ROLLBACK; -- Error occurred');
        logger.info(`Error: ${e.message}`);
        throw { message: e.message, logs }; 
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
    return withTransaction(async (client, logger) => {
        logger.raw(`\n> Processing: PAY, User: ${userId}, Amount: ${amount}\n`);
        logger.info(`[PAY] Starting transaction for User ${userId}.`);

        // 1. 檢查使用者
        logger.sql(`SELECT balance, credit_limit, current_points FROM Users WHERE user_id = ${userId};`);
        const user = await UserModel.getUserById(client, userId);
        if (!user) throw new Error('User not found');

        // 2. 檢查額度
        const newBalance = parseFloat(user.balance) + parseFloat(amount);
        const limit = parseFloat(user.credit_limit);
            if (newBalance > limit) {
            logger.info(`Balance check FAILED: Current(${user.balance}) + New(${amount}) = ${newBalance} > Limit(${limit}).`);
            throw new Error('Insufficient credit limit');
        }
        logger.info(`Balance check: Current(${user.balance}) + New(${amount}) = ${newBalance} <= Limit(${limit}). Passed.`);

        // 3. 計算點數
        const pointsEarned = Math.floor(amount * 0.01);
        logger.info(`Point calculation: ${amount} * 0.01 = ${pointsEarned} points.`);

        // 4. 寫入交易
        const maxId = await TxModel.getMaxTransactionId(client);
        const newTxId = maxId + 1;

        logger.sql(`INSERT INTO Transactions (transaction_id, user_id, amount, status, point_change) VALUES (${newTxId}, ${userId}, ${amount}, 'Paid', ${pointsEarned});`);
        await TxModel.createTransaction(client, {
            id: newTxId, userId, amount, status: 'Paid', pointChange: pointsEarned, sourceId: null
        });

        // 5. 更新使用者
        logger.sql(`UPDATE Users SET balance = balance + ${amount}, current_points = current_points + ${pointsEarned} WHERE user_id = ${userId};`);
        await UserModel.updateUserBalanceAndPoints(client, userId, amount, pointsEarned);

        logger.info(`Transaction ${newTxId} completed successfully.`);
        return { transactionId: newTxId, points: pointsEarned };
    });
};

export const voidTransaction = async (userId, targetTxId) => {
    return withTransaction(async (client, logger) => {
        logger.raw(`\n> Processing: VOID, Target Transaction: ${targetTxId}\n`);
        logger.info(`[VOID] Attempting to void transaction ${targetTxId}.`);

        logger.sql(`SELECT status, user_id, amount FROM Transactions WHERE transaction_id = ${targetTxId};`);
        const tx = await TxModel.getTransactionById(client, targetTxId);

        if (!tx) throw new Error('Transaction not found');
        if (Number(tx.user_id) !== Number(userId)) throw new Error('Security Alert: Unauthorized access.');

        if (tx.status !== 'Pending') {
            logger.info(`Check Failed: Status is '${tx.status}', expected 'Pending'.`);
            throw new Error(`Cannot void transaction with status: ${tx.status}`);
        }
        logger.info(`Transaction Status is 'Pending'. Action allowed.`);

        logger.sql(`UPDATE Transactions SET status = 'Voided' WHERE transaction_id = ${targetTxId};`);
        await TxModel.updateTransactionStatus(client, targetTxId, 'Voided');

        logger.sql(`UPDATE Users SET balance = balance - ${tx.amount} WHERE user_id = ${userId};`);
        await UserModel.updateUserBalanceAndPoints(client, userId, -tx.amount, 0);

        logger.info(`Transaction ${targetTxId} has been VOIDED. Balance restored.`);
        return { success: true };
    });
};

export const refundTransaction = async (userId, targetTxId) => {
    return withTransaction(async (client, logger) => {
        logger.raw(`\n> Processing: REFUND, Target Transaction: ${targetTxId}\n`);
        logger.info(`[REFUND] Attempting to refund transaction ${targetTxId}.`);

        logger.sql(`SELECT status, user_id, amount, point_change FROM Transactions WHERE transaction_id = ${targetTxId};`);
        const tx = await TxModel.getTransactionById(client, targetTxId);

        if (!tx) throw new Error('Transaction not found');
        if (Number(tx.user_id) !== Number(userId)) throw new Error('Security Alert: Unauthorized access.');

        if (tx.status !== 'Paid') throw new Error(`Cannot refund transaction with status: ${tx.status}`);

        logger.info(`Transaction is 'Paid'. Amount: ${tx.amount}, Points Generated: ${tx.point_change}.`);
        logger.info(`Verifying user has enough points to rollback.`);

        logger.sql(`SELECT current_points FROM Users WHERE user_id = ${userId};`);
        const user = await UserModel.getUserById(client, userId);

        if (user.current_points < tx.point_change) {
            logger.info(`Check Failed: User has ${user.current_points} points, needs ${tx.point_change}.`);
            throw new Error('Insufficient points to rollback transaction');
        }
        logger.info(`User has ${user.current_points} points. Deducting ${tx.point_change} points is safe.`);

        // 1. 標記原交易
        logger.sql(`UPDATE Transactions SET status = 'Refunded' WHERE transaction_id = ${targetTxId};`);
        await TxModel.updateTransactionStatus(client, targetTxId, 'Refunded');

        // 2. 補償交易
        const maxId = await TxModel.getMaxTransactionId(client);
        const refundTxId = maxId + 1;
        const refundAmount = -tx.amount;
        const refundPoints = -tx.point_change;

        logger.sql(`INSERT INTO Transactions VALUES (${refundTxId}, ${userId}, ${refundAmount}, 'Refunded', ${refundPoints}, ${targetTxId});`);
        await TxModel.createTransaction(client, {
            id: refundTxId, userId, amount: refundAmount, status: 'Refunded', pointChange: refundPoints, sourceId: targetTxId
        });

        // 3. 更新 User
        logger.sql(`UPDATE Users SET balance = balance - ${tx.amount}, current_points = current_points - ${tx.point_change} WHERE user_id = ${userId};`);
        await UserModel.updateUserBalanceAndPoints(client, userId, refundAmount, refundPoints);

        logger.info(`Refund processed. Compensating Transaction ${refundTxId} created.`);
        return { refundTransactionId: refundTxId };
    });
};