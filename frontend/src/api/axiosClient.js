import axios from 'axios'

const apiClient = axios.create({
    baseURL: 'http://localhost:3000/api',
    timeout: 5000,
    headers: {
        'Content-Type': 'application/json'
    }
})

// 回應攔截器，直接回傳 data
apiClient.interceptors.response.use(
    (response) => {
        return response.data
    },
    (error) => {
        // 可以在這裡統一處理 401, 403, 500 錯誤
        console.error('API Error:', error.response?.data || error.message)
        return Promise.reject(error)
    }
)

export default apiClient