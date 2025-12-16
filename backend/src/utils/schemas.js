import Joi from 'joi';

// 共用的 ID 驗證規則
const idSchema = Joi.number().integer().positive().required();

// 1. PAY 交易驗證規則
export const paySchema = Joi.object({
    user_id: idSchema,
    // 金額必須是數字、正數、最多兩位小數
    amount: Joi.number().positive().precision(2).required().messages({
        'number.base': '金額必須是數字',
        'number.positive': '金額必須大於 0',
        'number.precision': '金額最多只能有兩位小數'
    })
});

// 2. VOID 與 REFUND 的驗證規則
// 只需要 user_id 和 target_transaction_id
export const transactionActionSchema = Joi.object({
    user_id: idSchema,
    target_transaction_id: idSchema
});