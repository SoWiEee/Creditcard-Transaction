<script setup>
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
  logs: Array
})

const logContainer = ref(null)

// 當有新 Log 時自動捲動到底部
watch(() => props.logs.length, () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
})
</script>

<template>
  <div class="card log-container">
    <h3>系統後台日誌 (System Logs)</h3>
    <div class="log-window" ref="logContainer">
      <div v-for="(log, index) in logs" :key="index" class="log-line">
        <span class="log-time">[{{ log.time }}]</span>
        <span class="log-msg" :class="log.type">{{ log.message }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.log-window {
  flex: 1;
  background-color: #000;
  border-radius: 6px;
  padding: 10px;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  overflow-y: auto;
  max-height: 400px;
  border: 1px solid #333;
}

.log-line {
  margin-bottom: 4px;
}

.log-time {
  color: #64748b;
  margin-right: 10px;
}

/* Log 類型顏色 */
.info { color: #e2e8f0; }
.sql { color: #a5b4fc; } /* 紫色 SQL */
.error { color: #ef4444; }
.success { color: #10b981; }
</style>