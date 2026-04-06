FROM node:20-alpine

WORKDIR /app

# Install deps first for better cache
COPY package*.json ./
RUN npm install

# Copy app source into image (no bind mount needed)
COPY . .

EXPOSE 3000

# Start Vite dev server accessible from host
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
