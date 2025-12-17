<script setup>
defineProps({
  user: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <div class="stats-container">
    <div class="card stat-card">
      <div class="stat-label">Current Balance</div>
      <div class="stat-value balance">${{ Number(user.balance).toFixed(2) }}</div>
    </div>

    <div class="card stat-card">
      <div class="stat-label">Available Points</div>
      <div class="stat-value points">{{ user.current_points }} P</div>
    </div>

    <div class="card stat-card">
      <div class="stat-label">Credit Limit</div>
      <div class="stat-value limit">${{ user.credit_limit.toLocaleString() }}</div>
      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: (user.balance / user.credit_limit * 100) + '%' }"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stats-container {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
  margin-bottom: 20px;
}

.stat-label {
  color: var(--text-secondary);
  font-size: 0.9rem;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 1.8rem;
  font-weight: bold;
}

.points { color: var(--warning-color); }
.limit { color: var(--text-primary); }

.progress-bar {
  margin-top: 10px;
  height: 6px;
  background-color: #334155;
  border-radius: 3px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background-color: var(--accent-color);
  transition: width 0.3s ease;
}
</style>