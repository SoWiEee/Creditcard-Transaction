import pg from 'pg';

const { Pool } = pg;

let pool;

if (process.env.DATABASE_URL) {
  // Docker / production
  pool = new Pool({
    connectionString: process.env.DATABASE_URL,
  });
} else {
  // local dev
  pool = new Pool({
    user: process.env.DB_USER || 'cct_user',
    host: process.env.DB_HOST || 'localhost',
    database: process.env.DB_NAME || 'creditcard',
    password: process.env.DB_PASSWORD || 'cct_pass',
    port: process.env.DB_PORT || 5432,
  });
}

pool.on('connect', () => {
  console.log('[V] Database connected successfully');
});

export default pool;