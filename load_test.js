import http from 'k6/http';
import { check, sleep } from 'k6';

// 設定測試情境
export const options = {
    // 模擬 30 個使用者同時操作
    vus: 30, 
    // 持續轟炸 10 秒
    duration: '10s', 
    // 設定閾值：95% 的請求必須在 200ms 內完成
    thresholds: {
        http_req_duration: ['p(95)<200'], 
    },
};

export default function () {
    const url = 'http://backend:3000/api/transaction/pay';

    const payload = JSON.stringify({
        user_id: 1,       // 大家都針對 User 1
        amount: 100,      // 金額正常
        merchant: 'Steam',
        use_points: false,
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const res = http.post(url, payload, params);

    check(res, {
        'is success': (r) => r.status === 200 || r.status === 201,
        'is risk rejected': (r) => r.status === 500, 
        'fast response': (r) => r.timings.duration < 50,
    });

    sleep(Math.random() * 0.1); 
}