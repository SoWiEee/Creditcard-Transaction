<script setup>
import { ref, reactive } from 'vue'
import UserProfile from './components/UserProfile.vue'
import ActionPanel from './components/ActionPanel.vue'
import TransactionTable from './components/TransactionTable.vue'
import SystemLog from './components/SystemLog.vue'

// 1. Mock Data: 使用者資訊
const user = reactive({
  user_id: 1,
  name: "Alice",
  balance: 1000.00,
  credit_limit: 10000.00,
  current_points: 50
})

// 2. Mock Data: 交易紀錄
const transactions = ref([
  { transaction_id: 101, created_at: '2023-12-01T10:00:00', amount: 1000, status: 'Paid', point_change: 10 },
  { transaction_id: 103, created_at: '2023-12-03T09:15:00', amount: 200, status: 'Pending', point_change: 0 },
  { transaction_id: 104, created_at: '2023-12-03T10:00:00', amount: 500, status: 'Paid', point_change: 5 },
])

// 3. Mock Data: 日誌
const logs = ref([
  { time: '10:00:01', type: 'info', message: 'System initialized.' },
  { time: '10:00:02', type: 'sql', message: 'SELECT * FROM Users WHERE user_id = 1;' },
])

// --- 模擬後端邏輯 (之後會替換成 API 呼叫) ---

const addLog = (type, message) => {
  const time = new Date().toLocaleTimeString()
  logs.value.push({ time, type, message })
}

const onPay = (amount) => {
  addLog('info', `[PAY] Request received. Amount: $${amount}`)
  
  if (user.balance + amount > user.credit_limit) {
    addLog('error', 'Error: Insufficient credit limit.')
    return
  }

  // 模擬延遲
  setTimeout(() => {
    const points = Math.floor(amount * 0.01)
    const newId = transactions.value.length + 100 + 1
    
    // 模擬 SQL
    addLog('sql', `INSERT INTO Transactions VALUES (${newId}, ${amount}, 'Pending', ...);`)
    
    // 更新本地假資料
    transactions.value.unshift({
      transaction_id: newId,
      created_at: new Date().toISOString(),
      amount: Number(amount),
      status: 'Pending', // 預設 Pending 才能測試 Void
      point_change: 0
    })
    
    user.balance += Number(amount)
    addLog('success', `Transaction ${newId} created (Pending). Balance updated.`)
  }, 500)
}

const onVoid = (txId) => {
  addLog('info', `[VOID] Request received for Tx #${txId}`)
  
  const tx = transactions.value.find(t => t.transaction_id === txId)
  if (tx) {
    addLog('sql', `UPDATE Transactions SET status='Voided' WHERE id=${txId};`)
    addLog('sql', `UPDATE Users SET balance = balance - ${tx.amount};`)
    
    tx.status = 'Voided'
    user.balance -= tx.amount
    addLog('success', `Tx #${txId} voided successfully.`)
  }
}

const onRefund = (txId) => {
  addLog('info', `[REFUND] Request received for Tx #${txId}`)
  
  const tx = transactions.value.find(t => t.transaction_id === txId)
  // 假設點數足夠 (簡單模擬)
  const refundAmount = tx.amount
  const pointsToDeduct = tx.point_change

  addLog('sql', `INSERT INTO Transactions (Refund)... VALUES (-${refundAmount})...;`)
  
  // 更新原交易狀態
  tx.status = 'Refunded'
  
  // 新增一筆退款交易
  transactions.value.unshift({
    transaction_id: transactions.value.length + 200,
    created_at: new Date().toISOString(),
    amount: -refundAmount,
    status: 'Refunded',
    point_change: -pointsToDeduct,
    source: txId
  })

  user.balance -= refundAmount
  user.current_points -= pointsToDeduct
  addLog('success', `Refund processed. Points deducted: ${pointsToDeduct}`)
}
</script>

<template>
  <div class="app-container">
    <header>
      <h1>💳 Bank Dashboard System</h1>
      <p class="subtitle">Vue 3 + PostgreSQL Transaction Simulator</p>
    </header>

    <UserProfile :user="user" />
    
    <ActionPanel @on-pay="onPay" />

    <div class="main-grid">
      <TransactionTable 
        :transactions="transactions" 
        @on-void="onVoid"
        @on-refund="onRefund"
      />
      <SystemLog :logs="logs" />
    </div>
  </div>
</template>

<style scoped>
.app-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 40px 20px;
}

header {
  margin-bottom: 30px;
}

h1 {
  margin: 0;
  font-size: 2rem;
  background: linear-gradient(to right, #60a5fa, #a78bfa);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.subtitle {
  color: var(--text-secondary);
  margin-top: 5px;
}

/* 下方兩欄佈局 */
.main-grid {
  display: grid;
  grid-template-columns: 2fr 1fr; /* 左邊表格寬，右邊Log窄 */
  gap: 20px;
  align-items: start;
}

/* 響應式 */
@media (max-width: 768px) {
  .main-grid {
    grid-template-columns: 1fr;
  }
}
</style>