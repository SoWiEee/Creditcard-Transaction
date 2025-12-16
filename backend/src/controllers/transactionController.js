import * as TransactionService from '../services/transactionService.js';

export const pay = async (req, res) => {
  try {
    const { user_id, amount } = req.body;
    const result = await TransactionService.processPayment(user_id, amount);
    res.status(201).json(result);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
};

export const voidTx = async (req, res) => {
  try {
    const { user_id, target_transaction_id } = req.body;
    const result = await TransactionService.voidTransaction(user_id, target_transaction_id);
    res.json(result);
  } catch (error) {
    res.status(400).json({ error: error.message });
  }
};

export const refundTx = async (req, res) => {
  try {
    const { user_id, target_transaction_id } = req.body;
    const result = await TransactionService.refundTransaction(user_id, target_transaction_id);
    res.json(result);
  } catch (error) {
    // 如果是點數不足，通常回傳 409 Conflict，但在這裡統一簡單處理
    res.status(400).json({ error: error.message });
  }
};