import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

const MERCHANTS = ["7-11", "Steam", "Apple Store", "Amazon"];
const USERS = [1, 2, 3, 4];

const paySuccessRate = new Rate("pay_success_rate");     // 2xx
const riskRejectRate = new Rate("risk_reject_rate");     // 400
const serverErrorRate = new Rate("server_error_rate");   // 5xx

export const options = {
    scenarios: {
        steady: {
            executor: "constant-arrival-rate",
            rate: 6,
            timeUnit: "1s",
            duration: "2m",
            preAllocatedVUs: 20,
            maxVUs: 20,
        },
    },
    thresholds: {
        http_req_duration: ["p(95)<200"],
        // 只把真正的系統錯誤當失敗
        server_error_rate: ["rate<0.01"],     // 5xx < 1%
        // 風控拒絕：方案A你本來就會很高，這裡用「範圍」比較合理（可自行調）
        risk_reject_rate: ["rate>0.80"],      // 至少 80% 被擋（代表風控有效）
        // 成功率：方案A在你的規則下會很低
        pay_success_rate: ["rate>0.01"],      // 成功率至少 1%（避免全掛）
    },
};

export default function () {
    const user_id = USERS[randomIntBetween(0, USERS.length - 1)];
    const merchant = MERCHANTS[randomIntBetween(0, MERCHANTS.length - 1)];
    const amount = randomIntBetween(50, 800);
    const use_points = Math.random() < 0.25;

    const url = "http://backend:3000/api/transactions/pay";
    const payload = JSON.stringify({ user_id, amount, merchant, use_points });

    const res = http.post(url, payload, {
        headers: { "Content-Type": "application/json" },
        tags: { endpoint: "pay" },
    });

    const is2xx = res.status >= 200 && res.status < 300;
    const is400 = res.status === 400;
    const is5xx = res.status >= 500;

    paySuccessRate.add(is2xx);
    riskRejectRate.add(is400);
    serverErrorRate.add(is5xx);

    check(res, {
        "system alive (2xx/400)": (r) => (r.status >= 200 && r.status < 300) || r.status === 400,
        "fast (<50ms)": (r) => r.timings.duration < 50,
    });

    sleep(0.1);
}