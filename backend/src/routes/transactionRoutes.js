import * as txController from '../controllers/transactionController.js';
import { validate } from '../middleware/validate.js';
import { paySchema, transactionActionSchema } from '../utils/schemas.js';

export default async function transactionRoutes(fastify) {
    fastify.get('/health', async () => ({ status: 'ok' }));

    fastify.get('/users/:id', txController.getUserInfo);
    fastify.get('/transactions/:user_id', txController.getUserTransactions);

    fastify.post(
        '/transactions/pay',
        { preHandler: validate(paySchema) },
        txController.pay,
    );

    fastify.post(
        '/transactions/void',
        { preHandler: validate(transactionActionSchema) },
        txController.voidTx,
    );

    fastify.post(
        '/transactions/refund',
        { preHandler: validate(transactionActionSchema) },
        txController.refundTx,
    );
}
