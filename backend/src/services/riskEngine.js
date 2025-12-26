import pool from '../config/db.js';

// 風控規則常數設定
const RULES = {
    MAX_AMOUNT: 10000,      // 單筆上限
    MIN_AMOUNT: 1,          // 單筆下限
    VELOCITY_LIMIT: 3,      // 1分鐘內最多筆數
    VELOCITY_WINDOW: '1 minute',
    DUPLICATE_WINDOW: '5 minutes', // 重複交易檢查視窗
    REFUND_LIMIT: 3,        // 24小時內最多退款次數
    REFUND_WINDOW: '24 hours'
};

/**
 * 評估支付風險
 * @param {Object} client - DB Client
 * @param {number} userId - 使用者 ID
 * @param {number} amount - 交易金額
 * @param {string} merchant - 商家名稱
 * @param {Object} logger - 用來寫入 Log 的物件
 */
export const evaluatePaymentRisk = async (client, userId, amount, merchant, logger) => {
	logger.info(`[RISK] Starting Risk Evaluation for User ${userId}...`);

	// 規則 1 & 2: 金額限制
	if (amount > RULES.MAX_AMOUNT) {
		logger.info(`[RISK] FAIL: Amount $${amount} exceeds limit $${RULES.MAX_AMOUNT}.`);
		throw new Error(`Risk Control: Transaction amount exceeds maximum limit ($${RULES.MAX_AMOUNT}).`);
	}

	if (amount < RULES.MIN_AMOUNT) {
		logger.info(`[RISK] FAIL: Amount $${amount} is below minimum $${RULES.MIN_AMOUNT}.`);
		throw new Error(`Risk Control: Transaction amount is too low (Min: $${RULES.MIN_AMOUNT}).`);
	}

	logger.info('[RISK] PASS: Amount limits check.');

	// ---------------------------------------------------------
	// 規則 5: 退款濫用偵測 (Refund Abuse) -> 凍結帳戶
	// 檢查該使用者在過去 24 小時內的退款次數
	// ---------------------------------------------------------
	const refundRes = await client.query(
	`EXPLAIN (ANALYZE, BUFFERS)
		SELECT COUNT(*) as count FROM Transactions 
		WHERE user_id = $1 AND status = 'Refunded' 
		AND created_at > NOW() - INTERVAL '${RULES.REFUND_WINDOW}'`,
	[userId]
	);
	const refundCount = parseInt(refundRes.rows[0].count);

	if (refundCount >= RULES.REFUND_LIMIT) {
		logger.info(`[RISK] FAIL: User has ${refundCount} refunds in 24h. Account temporarily frozen.`);
	// 這裡我們直接拋出錯誤來模擬「凍結」效果，不允許進行新交易
	throw new Error(`Security Alert: Account temporarily frozen due to excessive refunds (${refundCount}/${RULES.REFUND_LIMIT} in 24h).`);
	}
	logger.info(`[RISK] PASS: Refund history check (${refundCount} refunds in 24h).`);

	// ---------------------------------------------------------
	// 規則 3: 同一 User ID 在 1 分鐘內 不得超過 3 筆 交易
	// ---------------------------------------------------------
	const velocityRes = await client.query(
	`EXPLAIN (ANALYZE, BUFFERS)
		SELECT COUNT(*) as count FROM Transactions 
		WHERE user_id = $1 
		AND created_at > NOW() - INTERVAL '${RULES.VELOCITY_WINDOW}'`,
	[userId]
	);
	const velocityCount = parseInt(velocityRes.rows[0].count);

	if (velocityCount >= RULES.VELOCITY_LIMIT) {
		logger.info(`[RISK] FAIL: Velocity limit reached (${velocityCount} tx in 1 min).`);
		throw new Error(`Risk Control: Too many transactions in short period. Please try again later.`);
	}
	logger.info(`[RISK] PASS: Velocity check (${velocityCount}/${RULES.VELOCITY_LIMIT} tx in 1 min).`);

	// ---------------------------------------------------------
	// 規則 4: 重複交易偵測 (Duplicate Transaction)
	// 同一 User, 同一 Merchant, 5 分鐘內, 相同金額
	// ---------------------------------------------------------
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
	logger.info('[RISK] PASS: Duplicate transaction check.');

	logger.info('[RISK] [V] All Risk Checks Passed.');
	return true;
};