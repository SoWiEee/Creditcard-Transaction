import express from 'express';
import cors from 'cors';
import transactionRoutes from './routes/transactionRoutes.js';

const app = express();
const PORT = 3000;

// Middleware
app.use(cors()); // 允許前端 Vue 呼叫
app.use(express.json());

// Routes
app.use('/api/transactions', transactionRoutes);

// Global Error Handler
app.use((err, req, res, next) => {
  console.error(err.stack);
  res.status(500).json({ error: 'Internal Server Error' });
});

app.listen(PORT, () => {
  console.log(`🚀 Server running on http://localhost:${PORT}`);
});