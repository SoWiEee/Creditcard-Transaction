<template>
	<div class="app-container">
		<header class="app-header">
			<div class="header-left">
				<h1>💳 Transaction System Simulator</h1>
				<p class="subtitle">Vue 3 + Node.js + PostgreSQL (ACID Demo)</p>
				</div>
				
				<div class="header-right">
				<label for="user-select">Current User:</label>
				<select id="user-select" v-model="currentUserId">
					<option :value="1">User 1 (Alice)</option>
					<option :value="2">User 2 (Bob)</option>
					<option :value="3">User 3 (Charlie)</option>
					<option :value="4">User 4 (David)</option>
				</select>
			</div>
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

<script setup>
import { ref, onMounted, reactive, watch } from 'vue'
import UserProfile from './components/UserProfile.vue'
import ActionPanel from './components/ActionPanel.vue'
import TransactionTable from './components/TransactionTable.vue'
import SystemLog from './components/SystemLog.vue'
import api from './api/transactionService'

const currentUserId = ref(1)

const loading = ref(false)
const user = ref({
	user_id: 1,
	name: 'Loading...',
	balance: 0,
	credit_limit: 0,
	current_points: 0
})
const transactions = ref([])
const logs = ref([])

const appendBackendLogs = (backendLogs) => {
	if (backendLogs && Array.isArray(backendLogs)) {
		backendLogs.forEach(msg => {
			// 根據訊息內容決定顏色 (簡單判斷)
			let type = 'info'
			if (msg.includes('[SQL]')) type = 'sql'
			if (msg.includes('Error')) type = 'error'
			
			logs.value.push({
			time: new Date().toLocaleTimeString(),
			type,
			message: msg
			})
		})
		// 加個分隔線
		logs.value.push({ time: '-', type: 'info', message: '--------------------------------------' })
	}
}

const addLog = (type, message) => {
	const time = new Date().toLocaleTimeString()
	logs.value.push({ time, type, message })
}

// fetch data
const refreshData = async () => {
	loading.value = true
	try {
		const [userData, txData] = await Promise.all([
			api.getUser(currentUserId.value),
			api.getTransactions(currentUserId.value)
		])
		user.value = userData
		transactions.value = txData
	} catch (error) {
		console.error(error)
		logs.value.push({ time: 'Error', type: 'error', message: '無法取得使用者資料' })
	} finally {
		loading.value = false
  }
}

// watch User ID
watch(currentUserId, () => {
	logs.value.push({ 
		time: new Date().toLocaleTimeString(), 
		type: 'info', 
		message: `[SYSTEM] Switched to User ${currentUserId.value}` 
	})
	refreshData()
})

// init
onMounted(() => {
	addLog('info', 'Frontend initialized. Connecting to backend...')
	refreshData()
})

// call API

const onPay = async (payload) => {
	const { amount, merchant, usePoints } = payload

	logs.value.push({ 
		time: new Date().toLocaleTimeString(), 
		type: 'info', 
		message: `action_type:PAY, user_id: ${currentUserId.value}, merchant: ${merchant}, amount: ${amount}, use_points: ${usePoints}` 
	})

	try {
		const res = await api.pay({ 
			user_id: currentUserId.value, 
			amount: Number(amount),
			merchant,
			use_points: usePoints
		})

		appendBackendLogs(res.logs) 
		await refreshData()
	} catch (error) {
		if (error.response?.data?.logs) {
			appendBackendLogs(error.response.data.logs)
		}
		const errMsg = error.response?.data?.error || error.message
		logs.value.push({ time: 'Error', type: 'error', message: `[PAY FAILED] ${errMsg}` })
	}
}

const onVoid = async (txId) => {
	logs.value.push({ 
		time: new Date().toLocaleTimeString(), 
		type: 'info', 
		message: `action_type:VOID, user_id: ${currentUserId.value}, target_tx: ${txId}` 
	})

	try {
		const res = await api.voidTx({ user_id: currentUserId.value, target_transaction_id: txId })
		appendBackendLogs(res.logs)
		await refreshData()
	} catch (error) {
		if (error.response?.data?.logs) appendBackendLogs(error.response.data.logs)
		const errMsg = error.response?.data?.error || error.message
		logs.value.push({ time: 'Error', type: 'error', message: `[VOID FAILED] ${errMsg}` })
	}
}

const onRefund = async (txId) => {
	logs.value.push({ 
		time: new Date().toLocaleTimeString(), 
		type: 'info', 
		message: `action_type:REFUND, user_id: ${currentUserId.value}, target_tx: ${txId}` 
	})

	try {
		const res = await api.refundTx({ user_id: currentUserId.value, target_transaction_id: txId })
		appendBackendLogs(res.logs)
		await refreshData()
	} catch (error) {
		if (error.response?.data?.logs) appendBackendLogs(error.response.data.logs)
		const errMsg = error.response?.data?.error || error.message
		logs.value.push({ time: 'Error', type: 'error', message: `[REFUND FAILED] ${errMsg}` })
	}
}
</script>

<style scoped>
.app-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 40px 20px;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
  padding-bottom: 20px;
  border-bottom: 1px solid #334155;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #1e293b;
  padding: 10px 20px;
  border-radius: 8px;
  border: 1px solid #334155;
}

select {
  background-color: #0f172a;
  color: white;
  border: 1px solid #475569;
  padding: 5px 10px;
  border-radius: 4px;
  font-size: 1rem;
  cursor: pointer;
}

h1 {
  margin: 0;
  font-size: 1.8rem;
  background: linear-gradient(to right, #60a5fa, #a78bfa);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.subtitle {
  color: #94a3b8;
  margin-top: 5px;
}

.main-grid {
  display: grid;
  grid-template-columns: 2fr 1.2fr; /* 調整比例讓 Log 區大一點 */
  gap: 20px;
  align-items: start;
}

@media (max-width: 768px) {
  .app-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 15px;
  }
  .main-grid {
    grid-template-columns: 1fr;
  }
}
</style>