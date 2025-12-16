import express from "express";
import cors from "cors";

const app = express();
const PORT = 3000;

/**
 * 基本 middleware
 */
app.use(express.json());
app.use(cors({
  origin: true,      // dev 階段先放寬
  credentials: true,
}));

/**
 * 健康檢查
 * GET /api/health
 */
app.get("/api/health", (req, res) => {
  res.json({
    status: "ok",
    time: new Date().toISOString(),
  });
});

/**
 * 看 Cloudflare / Proxy header
 * GET /api/ip
 */
app.get("/api/ip", (req, res) => {
  res.json({
    ip:
      req.headers["cf-connecting-ip"] ||
      req.headers["x-forwarded-for"] ||
      req.socket.remoteAddress,
    headers: {
      "cf-ray": req.headers["cf-ray"],
      "cf-country": req.headers["cf-ipcountry"],
      "user-agent": req.headers["user-agent"],
    },
  });
});

/**
 * 測試 POST
 * POST /api/echo
 */
app.post("/api/echo", (req, res) => {
  res.json({
    you_sent: req.body,
  });
});

app.listen(PORT, "127.0.0.1", () => {
  console.log(`API running at http://127.0.0.1:${PORT}`);
});
