<template>
  <transition name="slide-down">
    <div v-if="toast.visible" class="toast-wrap" role="alert" aria-live="polite">
      <div class="toast" :class="toast.type">
        <div class="title">{{ toast.title }}</div>
        <div class="msg">{{ toast.message }}</div>
        <button class="close" aria-label="Close" @click="toast.hide()">×</button>
      </div>
    </div>
  </transition>
</template>

<script setup>
import { useToastStore } from '../stores/toast'
const toast = useToastStore()
</script>

<style scoped>
.toast-wrap{
  position: fixed;
  top: 16px;
  left: 0;
  right: 0;
  z-index: 9999;
  display: flex;
  justify-content: center;
  pointer-events: none;
}
.toast{
  pointer-events: auto;
  min-width: 280px;
  max-width: 680px;
  border-radius: 12px;
  padding: 12px 14px;
  box-shadow: 0 10px 30px rgba(0,0,0,.12);
  background: #fff;
  border: 1px solid rgba(0,0,0,.08);
  position: relative;
}
.toast.warning{ border-left: 6px solid #f59e0b; }
.toast.error{ border-left: 6px solid #ef4444; }
.toast.success{ border-left: 6px solid #10b981; }
.toast.info{ border-left: 6px solid #3b82f6; }

.title{ font-weight: 700; margin-bottom: 4px; }
.msg{ opacity: .92; line-height: 1.4; padding-right: 20px; }

.close{
  position: absolute;
  top: 8px;
  right: 10px;
  background: transparent;
  border: 0;
  font-size: 18px;
  cursor: pointer;
  opacity: .6;
}
.close:hover{ opacity: 1; }

.slide-down-enter-active, .slide-down-leave-active{
  transition: transform .18s ease, opacity .18s ease;
}
.slide-down-enter-from, .slide-down-leave-to{
  transform: translateY(-16px);
  opacity: 0;
}
</style>
