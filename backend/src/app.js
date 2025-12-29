import Fastify from 'fastify';
import cors from '@fastify/cors';
import transactionRoutes from './routes/transactionRoutes.js';

const PORT = Number(process.env.PORT || 3000);

const fastify = Fastify({
  logger: false,
});

// CORS
await fastify.register(cors, {
	origin: true,
	credentials: true,
	methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
	allowedHeaders: ['Content-Type', 'Authorization'],
});

// routes
await fastify.register(transactionRoutes, { prefix: '/api' });

// 404 handler
fastify.setNotFoundHandler(async (request, reply) => {
	console.log(`[X] No route found for ${request.url}`);
	return reply.code(404).send({ error: 'Route not found', path: request.url });
});

try {
	await fastify.listen({ port: PORT, host: '0.0.0.0' });
	console.log(`🚀 Backend running on port ${PORT}`);
} catch (err) {
	console.error(err);
	process.exit(1);
}