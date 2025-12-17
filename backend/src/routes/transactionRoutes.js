import express from 'express';
import * as txController from '../controllers/transactionController.js';
import { validate } from '../middleware/validate.js';
import { paySchema, transactionActionSchema } from '../utils/schemas.js';

const router = express.Router();

router.get('/users/:id', txController.getUserInfo); 
router.get('/transactions/:user_id', txController.getUserTransactions);
router.post('/transactions/pay', validate(paySchema), txController.pay);
router.post('/transactions/void', validate(transactionActionSchema), txController.voidTx);
router.post('/transactions/refund', validate(transactionActionSchema), txController.refundTx);

export default router;