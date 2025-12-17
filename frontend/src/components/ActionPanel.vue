<script setup>
import { ref, computed, defineEmits } from 'vue'

const amount = ref(0)
const selectedMerchant = ref('7-11')
const usePoints = ref(false)
const emit = defineEmits(['on-pay'])

const merchants = [
	{ name: '7-11', multiplier: 1, label: '7-11 (x1 pts)' },
	{ name: 'Steam', multiplier: 2, label: 'Steam (x2 pts)' },
	{ name: 'Apple Store', multiplier: 3, label: 'Apple Store (x3 pts)' },
	{ name: 'Amazon', multiplier: 1.5, label: 'Amazon (x1.5 pts)' }
]

const handlePay = () => {
	if (amount.value <= 0) return alert('Amount must be > 0')

	emit('on-pay', {
		amount: amount.value,
		merchant: selectedMerchant.value,
		usePoints: usePoints.value
	})
}
</script>

<template>
	<div class="panel">
		<h2>🛍️ Make a Purchase</h2>
		<div class="form-group">
			<label>Merchant:</label>
				<select v-model="selectedMerchant">
				<option v-for="m in merchants" :key="m.name" :value="m.name">
					{{ m.label }}
				</option>
			</select>
		</div>

		<div class="form-group">
			<label>Amount ($):</label>
			<input type="number" v-model="amount" min="1" placeholder="0.00" />
		</div>

		<div class="form-group checkbox-group">
			<input type="checkbox" id="usePoints" v-model="usePoints" />
			<label for="usePoints">
			Use Points for Discount (100pts = $1)
			</label>
		</div>

		<button @click="handlePay" class="btn-pay">PAY</button>
	</div>
</template>

<style scoped>
.panel {
  background: #1e293b;
  padding: 20px;
  border-radius: 12px;
  border: 1px solid #334155;
  margin-bottom: 20px;
}
.form-group {
  margin-bottom: 15px;
}
label {
  display: block;
  color: #94a3b8;
  margin-bottom: 5px;
}
input[type="number"], select {
  width: 100%;
  padding: 10px;
  border-radius: 6px;
  border: 1px solid #475569;
  background: #0f172a;
  color: white;
}
.checkbox-group {
  display: flex;
  align-items: center;
  gap: 10px;
}
.checkbox-group label {
  margin-bottom: 0;
  cursor: pointer;
  color: #cbd5e1;
}
.btn-pay {
  width: 100%;
  padding: 12px;
  background: #10b981;
  color: white;
  border: none;
  border-radius: 6px;
  font-weight: bold;
  cursor: pointer;
  transition: 0.2s;
}
.btn-pay:hover {
  background: #059669;
}
</style>