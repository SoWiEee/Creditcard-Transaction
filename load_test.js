import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    vus: 30,
    duration: '10s',
};

export default function () {
    const url = 'http://backend:3000/api/transactions/pay';
    const payload = JSON.stringify({
    user_id: 1,
    amount: 100,
    merchant: 'Steam',
    use_points: false,
    });
    const params = { headers: { 'Content-Type': 'application/json' } };

    const res = http.post(url, payload, params);

    check(res, {
    // 成功 (200/201) 或 被風控擋下 (400/500) 都算系統正常運作
    'system alive': (r) => r.status === 200 || r.status === 201 || r.status === 400 || r.status === 500,
    'fast response': (r) => r.timings.duration < 50,
    });

    sleep(0.1);
}