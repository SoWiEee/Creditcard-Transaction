import * as TransactionService from '../services/transactionService.js';

export const getUserInfo = async (req, res) => {
	try {
		const userId = req.params.id;

		if (isNaN(userId)) {
			return res.status(400).json({ error: 'Invalid user ID format' });
		}

		const user = await TransactionService.getUserDetails(userId);
		res.json(user);

	} catch (error) {
		if (error.message === 'User not found') {
			res.status(404).json({ error: 'User not found' });
		} else {
			console.error(error);
			res.status(500).json({ error: 'Internal Server Error' });
		}
	}
};

export const getUserTransactions = async (req, res) => {
	try {
		const userId = req.params.user_id; 

		if (isNaN(userId)) {
			return res.status(400).json({ error: 'Invalid user ID format' });
		}

		const transactions = await TransactionService.getTransactionHistory(userId);
		res.json(transactions);

	} catch (error) {
		console.error(error);
		res.status(500).json({ error: 'Internal Server Error' });
	}
};

export const pay = async (req, res) => {
	try {
		const { user_id, amount } = req.body;
		const result = await TransactionService.processPayment(user_id, amount);
		res.status(201).json(result); 
	} catch (error) {
		res.status(400).json({ error: error.message, logs: error.logs || [] });
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
		res.status(400).json({ error: error.message, logs: error.logs || [] });
	}
};