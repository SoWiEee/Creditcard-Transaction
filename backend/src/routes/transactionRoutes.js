import express from 'express';
import * as txController from '../controllers/transactionController.js';

const router = express.Router();

router.post('/pay', txController.pay);
router.post('/void', txController.voidTx);
router.post('/refund', txController.refundTx);

export default router;