import express from 'express';
import cors from 'cors';
import transactionRoutes from './routes/transactionRoutes.js';

const app = express();
const PORT = 3000;

// Middleware
app.use((req, res, next) => {
  console.log(`[Incoming] ${req.method} ${req.url}`);
  next();
});

app.use(cors({
  origin: true,
  credentials: true,
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  allowedHeaders: ['Content-Type', 'Authorization']
}));
app.use(express.json());

// Routes
app.use('/api', transactionRoutes);

// 404 Handler
app.use((req, res) => {
  console.log(`[❌ 404] No route found for ${req.url}`);
  res.status(404).json({ error: 'Route not found', path: req.url });
});

app.listen(PORT, '0.0.0.0', () => {
  console.log(`🚀 Backend running on port ${PORT}`);
});