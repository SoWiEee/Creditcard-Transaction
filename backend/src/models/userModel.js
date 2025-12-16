export const getUserById = async (client, userId) => {
  const query = `
    SELECT user_id, username, balance, current_points, credit_limit 
    FROM Users WHERE user_id = $1
  `;
  const result = await client.query(query, [userId]);
  return result.rows[0];
};

export const updateUserBalanceAndPoints = async (client, userId, balanceChange, pointChange) => {
  const query = `
    UPDATE Users 
    SET balance = balance + $1, 
        current_points = current_points + $2
    WHERE user_id = $3
    RETURNING balance, current_points
  `;
  const result = await client.query(query, [balanceChange, pointChange, userId]);
  return result.rows[0];
};