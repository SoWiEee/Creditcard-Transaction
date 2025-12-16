<script setup>
defineProps({
  transactions: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['on-void', 'on-refund'])

const formatDate = (dateStr) => {
  return new Date(dateStr).toLocaleString()
}

// 狀態顏色 Helper
const getStatusClass = (status) => {
  switch (status) {
    case 'Paid': return 'status-paid'
    case 'Pending': return 'status-pending'
    case 'Voided': return 'status-voided'
    case 'Refunded': return 'status-refunded'
    default: return ''
  }
}
</script>

<template>
  <div class="card table-container">
    <h3>交易紀錄 (Transaction Ledger)</h3>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Date</th>
          <th>Amount</th>
          <th>Status</th>
          <th>Points</th>
          <th>Action</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="tx in transactions" :key="tx.transaction_id">
          <td>#{{ tx.transaction_id }}</td>
          <td>{{ formatDate(tx.created_at) }}</td>
          
          <td :class="{ 'text-refund': tx.amount < 0 }">
            {{ tx.amount < 0 ? '' : '$' }}{{ tx.amount }}
          </td>

          <td>
            <span class="badge" :class="getStatusClass(tx.status)">
              {{ tx.status }}
            </span>
          </td>
          
          <td :class="tx.point_change < 0 ? 'text-red' : 'text-green'">
            {{ tx.point_change > 0 ? '+' : '' }}{{ tx.point_change }}
          </td>

          <td>
            <button 
              v-if="tx.status === 'Pending'" 
              class="btn-void"
              @click="emit('on-void', tx.transaction_id)"
            >
              VOID
            </button>
            <button 
              v-if="tx.status === 'Paid'" 
              class="btn-refund"
              @click="emit('on-refund', tx.transaction_id)"
            >
              REFUND
            </button>
            <span v-if="['Voided', 'Refunded'].includes(tx.status)" class="text-muted">
              -
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.table-container {
  overflow-x: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th {
  text-align: left;
  color: var(--text-secondary);
  padding: 12px;
  border-bottom: 1px solid var(--border-color);
}

td {
  padding: 12px;
  border-bottom: 1px solid #334155;
}

/* 狀態標籤樣式 */
.badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.8rem;
  font-weight: bold;
}
.status-paid { background: rgba(16, 185, 129, 0.2); color: #34d399; }
.status-pending { background: rgba(59, 130, 246, 0.2); color: #60a5fa; }
.status-voided { background: rgba(148, 163, 184, 0.2); color: #94a3b8; text-decoration: line-through; }
.status-refunded { background: rgba(245, 158, 11, 0.2); color: #fbbf24; }

/* 按鈕樣式 */
.btn-void { background: var(--danger-color); color: white; padding: 6px 12px; font-size: 0.8rem; }
.btn-refund { background: var(--warning-color); color: #1e293b; padding: 6px 12px; font-size: 0.8rem; }

.text-refund { color: var(--success-color); }
.text-green { color: var(--success-color); }
.text-red { color: var(--danger-color); }
.text-muted { color: var(--text-secondary); }
</style>