import Joi from 'joi';

// 共用的 ID 驗證規則
const idSchema = Joi.number().integer().positive().required();

// 1. PAY 交易驗證規則
export const paySchema = Joi.object({
    user_id: idSchema,

    amount: Joi.number().positive().precision(2).required().messages({
        'number.base': '金額必須是數字',
        'number.positive': '金額必須大於 0',
        'number.precision': '金額最多只能有兩位小數'
    }),

    merchant: Joi.string().required().messages({
        'string.base': '商家名稱必須是文字',
        'any.required': '商家名稱為必填'
    }),

    use_points: Joi.boolean().optional()
});

// 2. VOID 與 REFUND 的驗證規則
export const transactionActionSchema = Joi.object({
    user_id: idSchema,
    target_transaction_id: idSchema
});