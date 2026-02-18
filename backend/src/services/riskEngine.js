import redis from '../config/redis.js';

const RULES = {
    MAX_AMOUNT: 10000,              // 單筆上限
    MIN_AMOUNT: 1,                  // 單筆下限
    VELOCITY_LIMIT: 3,              // 1 分鐘內最多筆數
    VELOCITY_WINDOW: '1 minute',
    VELOCITY_WINDOW_SEC: 60,        // Redis TTL
    DUPLICATE_WINDOW: '5 minutes',  // 重複交易檢查視窗
    REFUND_LIMIT: 3,                // 24 小時內最多退款次數
    REFUND_WINDOW: '24 hours'
};

// redis quick check
export const evaluateVelocity = async (userId, logger) => {
    const velocityKey = `risk:velocity:user:${userId}`;
    const currentCount = await redis.incr(velocityKey);

    if (currentCount === 1) {
        await redis.expire(velocityKey, RULES.VELOCITY_WINDOW_SEC);
    }

    if (currentCount > RULES.VELOCITY_LIMIT) {
        logger.info(`[RISK] FAIL: Velocity limit reached (Redis: ${currentCount}).`);
        throw new Error(`Risk Control: Too many transactions. Please try again later.`);
    }
    logger.info(`[RISK] PASS: Velocity check (Redis: ${currentCount}/${RULES.VELOCITY_LIMIT}).`);
};

export const evaluatePaymentRisk = async (client, userId, amount, merchant, logger) => {
    logger.info(`[RISK] Starting Risk Evaluation for User ${userId}...`);

    // 1. 金額限制
    if (amount > RULES.MAX_AMOUNT) {
        logger.info(`[RISK] FAIL: Amount $${amount} exceeds limit $${RULES.MAX_AMOUNT}.`);
        throw new Error(`Risk Control: Transaction amount exceeds maximum limit ($${RULES.MAX_AMOUNT}).`);
    }

    if (amount < RULES.MIN_AMOUNT) {
        logger.info(`[RISK] FAIL: Amount $${amount} is below minimum $${RULES.MIN_AMOUNT}.`);
        throw new Error(`Risk Control: Transaction amount is too low (Min: $${RULES.MIN_AMOUNT}).`);
    }

    // 2. 使用者在過去 24 小時內的退款次數
    const refundRes = await client.query(
        `SELECT COUNT(*) as count FROM Transactions 
         WHERE user_id = $1 AND status = 'Refunded' 
         AND created_at > NOW() - INTERVAL '${RULES.REFUND_WINDOW}'`,
        [userId]
    );
    const refundCount = parseInt(refundRes.rows[0].count);

    if (refundCount >= RULES.REFUND_LIMIT) {
        logger.info(`[RISK] FAIL: User has ${refundCount} refunds in 24h. Account temporarily frozen.`);
        throw new Error(`Security Alert: Account temporarily frozen due to excessive refunds (${refundCount}/${RULES.REFUND_LIMIT} in 24h).`);
    }

    // 3. 同一 User, 同一 Merchant, 5 分鐘內, 相同金額
    const duplicateRes = await client.query(
        `SELECT COUNT(*) as count FROM Transactions 
         WHERE user_id = $1 
         AND merchant = $2 
         AND amount = $3 
         AND created_at > NOW() - INTERVAL '${RULES.DUPLICATE_WINDOW}'`,
        [userId, merchant, amount]
    );
    const duplicateCount = parseInt(duplicateRes.rows[0].count);

    if (duplicateCount > 0) {
        logger.info(`[RISK] FAIL: Duplicate transaction detected (Same amount to ${merchant} in 5 min).`);
        throw new Error(`Risk Control: Potential duplicate transaction detected.`);
    }

    logger.info('[RISK] [V] All Risk Checks Passed.');
    return true;
};