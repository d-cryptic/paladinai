FROM node:18-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy application files
COPY server.js ./

EXPOSE 4000

CMD ["node", "server.js"]