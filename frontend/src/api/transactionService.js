import apiClient from './axiosClient'

export default {
    // 1. 取得使用者資訊 (Dashboard)
    getUser(userId) {
        return apiClient.get(`/users/${userId}`)
    },

    // 2. 取得交易紀錄
    getTransactions(userId) {
        return apiClient.get(`/transactions/${userId}`)
    },

    // 3. 發起交易 (PAY)
    pay(payload) {
    // payload: { user_id, amount }
        return apiClient.post('/transactions/pay', payload)
    },

    // 4. 作廢交易 (VOID)
    voidTx(payload) {
    // payload: { user_id, target_transaction_id }
        return apiClient.post('/transactions/void', payload)
    },

    // 5. 退款交易 (REFUND)
    refundTx(payload) {
    // payload: { user_id, target_transaction_id }
        return apiClient.post('/transactions/refund', payload)
    }
}