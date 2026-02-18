import axios from 'axios'
import { useToastStore } from '../stores/toast'

const apiClient = axios.create({
  baseURL: 'http://localhost:3000/api',
  timeout: 5000,
  headers: { 'Content-Type': 'application/json' },
})

// 避免短時間大量請求把 toast 洗爆
const toastThrottle = {
  lastKey: '',
  lastAt: 0,
  windowMs: 800,
  allow(key) {
    const now = Date.now()
    if (this.lastKey === key && now - this.lastAt < this.windowMs) return false
    this.lastKey = key
    this.lastAt = now
    return true
  },
}

const RISK_CODE_TO_MESSAGE = {
  RISK_VELOCITY_LIMIT: '交易過於頻繁，請稍後再試（1 分鐘內最多 3 筆）。',
  RISK_DUPLICATE: '疑似重複交易：相同商家/金額在短時間內重複送出。',
  RISK_REFUND_ABUSE: '退款次數過多，帳戶暫時被保護性凍結。',
  RISK_AMOUNT_TOO_HIGH: '單筆金額超過上限，請檢查後再試。',
  RISK_AMOUNT_TOO_LOW: '單筆金額過低，請檢查後再試。',
  REDIS_UNAVAILABLE: '風控系統暫時不可用，請稍後再試。',
}

const CODE_TO_TOAST = {
  // Request / validation
  BAD_JSON: { title: '請求格式錯誤', message: '請檢查輸入格式（JSON 解析失敗）。', type: 'error' },
  VALIDATION_FAILED: { title: '資料驗證失敗', message: '請檢查必填欄位與金額（user_id / amount / merchant）。', type: 'error' },
  INVALID_USER_ID: { title: '輸入錯誤', message: '使用者 ID 格式不正確。', type: 'error' },
  INVALID_MERCHANT: { title: '輸入錯誤', message: '商家名稱不合法（僅允許 7-11 / Steam / Apple Store / Amazon）。', type: 'error' },

  // Resource
  USER_NOT_FOUND: { title: '找不到使用者', message: '查無此使用者，請確認 user_id。', type: 'error' },
  TX_NOT_FOUND: { title: '找不到交易', message: '查無此交易，請刷新列表後再試。', type: 'error' },

  // Permission / status
  TX_FORBIDDEN: { title: '無權限', message: '你沒有權限操作這筆交易。', type: 'error' },
  TX_INVALID_STATUS: { title: '操作不允許', message: '此交易狀態不允許執行此操作。', type: 'error' },

  // Business
  INSUFFICIENT_CREDIT: { title: '交易失敗', message: '可用額度不足，請降低金額或先還款。', type: 'error' },
  INSUFFICIENT_POINTS: { title: '交易失敗', message: '點數不足，無法回滾此交易的點數。', type: 'error' },

  // Internal
  INTERNAL_ERROR: { title: '系統錯誤', message: '伺服器忙碌或發生錯誤，請稍後再試。', type: 'error' },
}

function getErrorText(data) {
  const e = data?.error
  if (!e) return ''
  if (typeof e === 'string') return e
  return String(e)
}

// fallback（當 code 缺失時才用字串推論）
function inferRiskMessageByText(data) {
  const raw = getErrorText(data)
  if (!raw) return null
  const msg = raw.toLowerCase()

  if (msg.includes('too many transactions') || msg.includes('velocity')) return RISK_CODE_TO_MESSAGE.RISK_VELOCITY_LIMIT
  if (msg.includes('duplicate')) return RISK_CODE_TO_MESSAGE.RISK_DUPLICATE
  if (msg.includes('excessive refunds') || msg.includes('frozen')) return RISK_CODE_TO_MESSAGE.RISK_REFUND_ABUSE
  if (msg.includes('exceeds') && msg.includes('maximum')) return RISK_CODE_TO_MESSAGE.RISK_AMOUNT_TOO_HIGH
  if (msg.includes('too low') || msg.includes('below minimum')) return RISK_CODE_TO_MESSAGE.RISK_AMOUNT_TOO_LOW

  if (msg.includes('risk')) return raw
  return null
}

function mapErrorToToast(status, data) {
  const code = data?.code
  const raw = getErrorText(data)

  // 1) Risk codes（warning）
  if (code && RISK_CODE_TO_MESSAGE[code]) {
    return { title: '風險控制', message: RISK_CODE_TO_MESSAGE[code], type: 'warning', key: `risk:${code}` }
  }

  // 2) General known codes
  if (code && CODE_TO_TOAST[code]) {
    return { ...CODE_TO_TOAST[code], key: `code:${code}` }
  }

  // 3) Fallback inference (old backend / unexpected)
  const inferredRisk = inferRiskMessageByText(data)
  if (inferredRisk) {
    return { title: '風險控制', message: inferredRisk, type: 'warning', key: 'risk:inferred' }
  }

  // 4) Status-based fallback
  if (status === 404) return { title: '找不到資料', message: raw || '查無資料。', type: 'error', key: 'http:404' }
  if (status === 429) return { title: '請稍後再試', message: raw || '請求過於頻繁，請稍後再試。', type: 'warning', key: 'http:429' }
  if (status >= 500) return { title: '系統錯誤', message: '伺服器忙碌或發生錯誤，請稍後再試。', type: 'error', key: 'http:5xx' }

  // 5) Final fallback show raw error
  return { title: '操作失敗', message: raw || '請求失敗，請稍後再試。', type: 'error', key: 'fallback' }
}

apiClient.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    const toast = useToastStore()
    const status = error?.response?.status
    const data = error?.response?.data

    const t = mapErrorToToast(status, data)
    if (t && toastThrottle.allow(t.key || `${status || 'x'}:${data?.code || ''}:${getErrorText(data)}`)) {
      toast.show({ title: t.title, message: t.message, type: t.type, duration: 2000 })
    }

    // Debug logs（可選）
    if (data?.logs?.length) console.debug('API logs:', data.logs)
    console.error('API Error:', data || error.message)

    return Promise.reject(error)
  }
)

export default apiClient