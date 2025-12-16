<script setup>
import { ref, onMounted, reactive } from 'vue'
import UserProfile from './components/UserProfile.vue'
import ActionPanel from './components/ActionPanel.vue'
import TransactionTable from './components/TransactionTable.vue'
import SystemLog from './components/SystemLog.vue'
import api from './api/transactionService' // 引入剛寫好的 Service

// 目前模擬登入的使用者 ID (固定為 1，之後可做登入頁)
const CURRENT_USER_ID = 1

// 狀態管理
const loading = ref(false)
const user = ref({
  user_id: CURRENT_USER_ID,
  name: 'Loading...',
  balance: 0,
  credit_limit: 0,
  current_points: 0
})
const transactions = ref([])
const logs = ref([])

const addLog = (type, message) => {
  const time = new Date().toLocaleTimeString()
  logs.value.push({ time, type, message })
}

// 核心資料獲取函式
const refreshData = async () => {
  loading.value = true
  try {
    // 並行請求，加快速度
    const [userData, txData] = await Promise.all([
      api.getUser(CURRENT_USER_ID),
      api.getTransactions(CURRENT_USER_ID)
    ])
    
    user.value = userData
    transactions.value = txData // 假設後端回傳的是陣列
    
  } catch (error) {
    addLog('error', `Data Refresh Failed: ${error.message}`)
  } finally {
    loading.value = false
  }
}

// init
onMounted(() => {
  addLog('info', 'Frontend initialized. Connecting to backend...')
  refreshData()
})

// call API

const onPay = async (amount) => {
  addLog('info', `[PAY] Sending request... Amount: $${amount}`)
  
  try {
    const payload = {
      user_id: CURRENT_USER_ID,
      amount: Number(amount)
    }
    
    const res = await api.pay(payload)
    
    addLog('success', `[PAY] Success! Tx ID: ${res.transactionId}, Points: +${res.points}`)
    await refreshData() // 刷新餘額與列表

  } catch (error) {
    const errMsg = error.response?.data?.error || error.message
    // 如果是後端驗證失敗 (例如 Joi 擋下的)，顯示詳細錯誤
    const details = error.response?.data?.errors ? ` (${error.response.data.errors.join(', ')})` : ''
    addLog('error', `[PAY] Failed: ${errMsg}${details}`)
  }
}

const onVoid = async (txId) => {
  addLog('info', `[VOID] Requesting void for Tx #${txId}...`)
  
  try {
    const payload = {
      user_id: CURRENT_USER_ID,
      target_transaction_id: txId
    }
    
    await api.voidTx(payload)
    
    addLog('success', `[VOID] Tx #${txId} voided successfully.`)
    await refreshData()

  } catch (error) {
    const errMsg = error.response?.data?.error || error.message
    addLog('error', `[VOID] Failed: ${errMsg}`)
  }
}

const onRefund = async (txId) => {
  addLog('info', `[REFUND] Requesting refund for Tx #${txId}...`)
  
  try {
    const payload = {
      user_id: CURRENT_USER_ID,
      target_transaction_id: txId
    }
    
    const res = await api.refundTx(payload)
    
    addLog('success', `[REFUND] Success. New Refund Tx ID: ${res.refundTransactionId}`)
    await refreshData()

  } catch (error) {
    const errMsg = error.response?.data?.error || error.message
    addLog('error', `[REFUND] Failed: ${errMsg}`)
  }
}
</script>