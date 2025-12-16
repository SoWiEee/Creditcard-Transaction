import express from 'express';
import * as txController from '../controllers/transactionController.js';
import { validate } from '../middleware/validate.js';
import { paySchema, transactionActionSchema } from '../utils/schemas.js';

const router = express.Router();

router.post('/pay', validate(paySchema), txController.pay);
router.post('/void', validate(transactionActionSchema), txController.voidTx);
router.post('/refund', validate(transactionActionSchema), txController.refundTx);

export default router;