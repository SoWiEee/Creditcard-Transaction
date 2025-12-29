import * as TransactionService from '../services/transactionService.js';

export const getUserInfo = async (request, reply) => {
	try {
		const userId = request.params.id;

		if (isNaN(userId)) {
			return reply.code(400).send({ error: 'Invalid user ID format' });
		}

		const user = await TransactionService.getUserDetails(userId);
		return user;

	} catch (error) {
		if (error.message === 'User not found') {
			return reply.code(404).send({ error: 'User not found' });
		}

		console.error(error);
		return reply.code(500).send({ error: 'Internal Server Error' });
	}
};

export const getUserTransactions = async (request, reply) => {
	try {
		const userId = request.params.user_id;

		if (isNaN(userId)) {
			return reply.code(400).send({ error: 'Invalid user ID format' });
		}

		const transactions = await TransactionService.getTransactionHistory(userId);
		return transactions;

	} catch (error) {
		console.error(error);
		return reply.code(500).send({ error: 'Internal Server Error' });
	}
};

export const pay = async (request, reply) => {
	try {
		const { user_id, amount, merchant, use_points } = request.body;

		const result = await TransactionService.processPayment(
			user_id,
			amount,
			merchant || 'Unknown',
			use_points || false,
		);
		return reply.code(201).send(result);
	} catch (error) {
		return reply.code(400).send({ error: error.message, logs: error.logs || [] });
	}
};

export const voidTx = async (request, reply) => {
	try {
		const { user_id, target_transaction_id } = request.body;
		const result = await TransactionService.voidTransaction(user_id, target_transaction_id);
		return result;
	} catch (error) {
		return reply.code(400).send({ error: error.message });
	}
};

export const refundTx = async (request, reply) => {
	try {
		const { user_id, target_transaction_id } = request.body;
		const result = await TransactionService.refundTransaction(user_id, target_transaction_id);
		return result;
	} catch (error) {
		return reply.code(400).send({ error: error.message, logs: error.logs || [] });
	}
};