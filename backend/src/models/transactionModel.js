export const createTransaction = async (client, { id, userId, amount, status, pointChange, sourceId }) => {
	const query = `
	INSERT INTO Transactions 
	(transaction_id, user_id, amount, status, point_change, source_transaction_id)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING transaction_id
	`;
	await client.query(query, [id, userId, amount, status, pointChange, sourceId]);
};

export const getTransactionById = async (client, transactionId) => {
	const query = `SELECT * FROM Transactions WHERE transaction_id = $1 FOR UPDATE`; 
	// 注意：FOR UPDATE 用於鎖定該行，避免並發問題
	const result = await client.query(query, [transactionId]);
	return result.rows[0];
};

// 取得特定使用者的所有交易，按時間倒序排列
export const getTransactionsByUserId = async (client, userId) => {
  const query = `
    SELECT 
      transaction_id, 
      user_id, 
      amount, 
      status, 
      point_change, 
      created_at,
      source_transaction_id
    FROM Transactions 
    WHERE user_id = $1 
    ORDER BY created_at DESC
  `;
  const result = await client.query(query, [userId]);
  return result.rows;
};

export const updateTransactionStatus = async (client, transactionId, newStatus) => {
	const query = `UPDATE Transactions SET status = $1 WHERE transaction_id = $2`;
	await client.query(query, [newStatus, transactionId]);
};

// 取得最新 ID 用於生成流水號 (簡單實作)
export const getMaxTransactionId = async (client) => {
	const res = await client.query('SELECT MAX(transaction_id) as max_id FROM Transactions');
	return res.rows[0].max_id || 100;
}