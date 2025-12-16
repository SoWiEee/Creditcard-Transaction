<script setup>
import { ref } from 'vue'

const amount = ref(null)
const emit = defineEmits(['on-pay'])

const handlePay = () => {
  if (!amount.value || amount.value <= 0) return
  emit('on-pay', amount.value)
  amount.value = null // 清空輸入框
}
</script>

<template>
  <div class="card action-panel">
    <h3>發起新交易 (New Transaction)</h3>
    <div class="input-group">
      <input 
        type="number" 
        v-model="amount" 
        placeholder="輸入金額 (例如: 300)"
        @keyup.enter="handlePay"
      >
      <button class="btn-pay" @click="handlePay">PAY (支付)</button>
    </div>
  </div>
</template>

<style scoped>
.action-panel {
  margin-bottom: 20px;
}

.input-group {
  display: flex;
  gap: 15px;
}

input {
  flex: 1;
  background-color: #0f172a;
  border: 1px solid var(--border-color);
  color: white;
  padding: 12px;
  border-radius: 6px;
  font-size: 1rem;
}

.btn-pay {
  background-color: var(--success-color);
  color: white;
  padding: 0 30px;
  font-size: 1rem;
}
</style>