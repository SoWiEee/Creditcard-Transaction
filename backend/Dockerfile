FROM oven/bun:1.0

WORKDIR /app
COPY package.json bun.lock ./
RUN bun install

COPY . .

RUN chmod +x start.sh

EXPOSE 3000
CMD ["./start.sh"]