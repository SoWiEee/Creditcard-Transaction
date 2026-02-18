import pool from '../config/db.js';
import * as UserModel from '../models/userModel.js';
import * as TxModel from '../models/transactionModel.js';
import * as RiskEngine from './riskEngine.js';

const MERCHANT_RATES = {
    '7-11': 1,         // x1
    'Steam': 2,        // x2
    'Apple Store': 3,  // x3
    'Amazon': 1.5      // x1.5
};

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

export const processPayment = async (userId, amount, merchant, usePoints) => {
    // redis cache risk
    const preCheckLogger = { info: console.log, raw: () => {} };
    await RiskEngine.evaluateVelocity(userId, preCheckLogger);

    return withTransaction(async (client, logger) => {
        logger.raw(`\n> Processing: PAY at ${merchant}, User: ${userId}, Total: $${amount}\n`);

        await RiskEngine.evaluatePaymentRisk(client, userId, amount, merchant, logger);

        // 1. 取得使用者資料
        logger.info(`[PAY] Starting transaction logic for User ${userId}.`);
        logger.sql(`SELECT * FROM Users WHERE user_id = ${userId} FOR UPDATE;`); 

        const res = await client.query('SELECT * FROM Users WHERE user_id = $1 FOR UPDATE', [userId]);
        const user = res.rows[0];
        if (!user) throw new Error('User not found');

        let finalAmount = parseFloat(amount);
        let pointsRedeemed = 0;
        let discountAmount = 0;

        // 2. 計算點數折抵 (100 pts = $1)
        if (usePoints && user.current_points >= 100) {
            logger.info(`[Points Redemption] User has ${user.current_points} pts. Calculating discount...`);
            
            // 計算最大可折抵金額 (不能超過交易金額)
            const maxDiscount = Math.min(Math.floor(user.current_points / 100), Math.floor(finalAmount));
            
            if (maxDiscount > 0) {
                pointsRedeemed = maxDiscount * 100;
                discountAmount = maxDiscount;
                finalAmount = finalAmount - discountAmount;
                
                logger.info(`Redeeming ${pointsRedeemed} pts for $${discountAmount} discount.`);
            } else {
                logger.info(`Points insufficient for minimum $1 discount or amount is too small.`);
            }
        } else {
            logger.info(`No points redemption applied.`);
        }

        logger.info(`Final Payment: $${amount} - $${discountAmount} (Points) = $${finalAmount} (Cash)`);

        // 3. 檢查現金餘額
        const userBalance = parseFloat(user.balance);
        const userLimit = parseFloat(user.credit_limit);
        if ((userBalance + finalAmount) > userLimit) {
            throw new Error(`Insufficient credit. New Balance ${userBalance + finalAmount} > Limit ${userLimit}`);
        }

        // 4. 計算回饋點數 (只針對現金支付部分回饋)
        const multiplier = MERCHANT_RATES[merchant] || 1;
        // 規則：每 $1 現金回饋 1 點 * 倍率
        const pointsEarned = Math.floor(finalAmount * 1 * multiplier);

        logger.info(`[Rewards] Merchant: ${merchant} (x${multiplier}). Points Earned: ${Math.floor(finalAmount)} * ${multiplier} = ${pointsEarned}.`);
        

        // A. 建立交易紀錄
        const maxIdRes = await client.query('SELECT MAX(transaction_id) FROM Transactions');
        const newTxId = (maxIdRes.rows[0].max || 0) + 1;
        const netPointChange = pointsEarned - pointsRedeemed; // 總變動

        logger.sql(`INSERT INTO Transactions (id, user, merchant, amount, status, point_change) VALUES (${newTxId}, ${userId}, '${merchant}', ${finalAmount}, 'Paid', ${netPointChange});`);

        await client.query(
            `INSERT INTO Transactions (transaction_id, user_id, amount, status, point_change, merchant) 
            VALUES ($1, $2, $3, 'Paid', $4, $5)`,
            [newTxId, userId, finalAmount, netPointChange, merchant]
        );

        // B. 紀錄點數流水 (PointLogs) - 如果有消耗
        if (pointsRedeemed > 0) {
            logger.sql(`INSERT INTO Points (Redeemed: -${pointsRedeemed});`);
            await client.query(
                `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1, $2, $3, $4)`,
                [userId, newTxId, -pointsRedeemed, 'Redeemed']
            );
        }

        // C. 紀錄點數流水
        if (pointsEarned > 0) {
            logger.sql(`INSERT INTO PointLogs (Earned: +${pointsEarned});`);
            await client.query(
                `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1, $2, $3, $4)`,
                [userId, newTxId, pointsEarned, `Earned (${merchant} x${multiplier})`]
            );
        }

        // D. 更新使用者餘額與總點數
        logger.sql(`UPDATE Users SET balance = balance + ${finalAmount}, current_points = current_points + ${netPointChange} WHERE user_id = ${userId};`);
        await client.query(
            `UPDATE Users SET balance = balance + $1, current_points = current_points + $2 WHERE user_id = $3`,
            [finalAmount, netPointChange, userId]
        );

        logger.info(`Transaction ${newTxId} completed. Net Points: ${netPointChange > 0 ? '+' : ''}${netPointChange}`);
        return { transactionId: newTxId, finalAmount, pointsEarned, pointsRedeemed };
    });
};

export const voidTransaction = async (userId, targetTxId) => {
    return withTransaction(async (client, logger) => {
        logger.raw(`\n> Processing: VOID, Target Transaction: ${targetTxId}\n`);
        logger.info(`[VOID] Attempting to void transaction ${targetTxId}.`);

        // 1. 查詢目標交易 (包含 amount 和 point_change)
        logger.sql(`SELECT status, user_id, amount, point_change, merchant FROM Transactions WHERE transaction_id = ${targetTxId};`);

        // 注意：這裡我們假設 transactionModel 有正確回傳 point_change 欄位
        // 如果您之前的 Model 是寫 SELECT *，那通常都已經包含了
        const tx = await TxModel.getTransactionById(client, targetTxId);

        if (!tx) throw new Error('Transaction not found');

        // 安全檢查
        if (Number(tx.user_id) !== Number(userId)) {
            throw new Error('Security Alert: Unauthorized access. You do not own this transaction.');
        }

        // 狀態檢查 (通常 Void 只能針對 Pending，或是當日交易，這裡假設 Pending/Paid 依業務邏輯而定)
        // 為了測試方便，我們允許 Void 'Paid' 的交易 (模擬當日作廢)
        if (tx.status === 'Voided' || tx.status === 'Refunded') {
            throw new Error(`Cannot void transaction with status: ${tx.status}`);
        }

        logger.info(`Transaction Status is '${tx.status}'. Amount: $${tx.amount}, Points Net Change: ${tx.point_change}.`);

        // --- 開始回滾 ---

        // 2. 更新交易狀態
        logger.sql(`UPDATE Transactions SET status = 'Voided' WHERE transaction_id = ${targetTxId};`);
        await TxModel.updateTransactionStatus(client, targetTxId, 'Voided');

        // 3. 回滾現金 (Balance)
        // 當初是扣錢 (-amount)，現在要加回去 (+amount)
        logger.info(`Restoring Balance: +$${tx.amount}`);
        logger.sql(`UPDATE Users SET balance = balance + ${tx.amount} WHERE user_id = ${userId};`);
        await UserModel.updateUserBalanceAndPoints(client, userId, tx.amount, 0); // 先只還錢，點數下面處理

        // 4. 回滾點數 (Points)
        // 邏輯：原本的 point_change 如果是 +50 (賺到)，我們現在要 -50。
        //      原本的 point_change 如果是 -100 (折抵)，我們現在要 +100 (還給他)。
        //      所以直接取 "負號" 即可。
        const reversePointChange = -1 * tx.point_change;

        if (reversePointChange !== 0) {
            logger.info(`Restoring Points: ${reversePointChange > 0 ? '+' : ''}${reversePointChange}`);
            
            // 更新 User 點數
            logger.sql(`UPDATE Users SET current_points = current_points + ${reversePointChange} WHERE user_id = ${userId};`);
            // 這裡我們直接用 SQL 更新點數，因為 userModel.updateUserBalanceAndPoints 可能設計成同時更新
            // 為了簡單，我們再次呼叫 model，但這次金額是 0，只更新點數
            await UserModel.updateUserBalanceAndPoints(client, userId, 0, reversePointChange);

            // 寫入 Points
            logger.sql(`INSERT INTO Points (Void Reversal: ${reversePointChange});`);
            await client.query(
                `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1, $2, $3, $4)`,
                [userId, targetTxId, reversePointChange, 'Void Reversal']
            );
        } else {
            logger.info('No point change involved in this transaction.');
        }

        logger.info(`Transaction ${targetTxId} has been VOIDED. Cash & Points restored.`);
        return { success: true, voidedAmount: tx.amount, restoredPoints: reversePointChange };
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