FROM node:20-alpine

# Set the working directory
WORKDIR /app

# Copy package files for dependency installation
COPY package*.json ./

# Install dependencies
RUN npm install

# Note: Source code will be mounted via volume in docker-compose
# This allows hot reloading without rebuilding the image

# Expose the port the Vite dev server runs on
EXPOSE 5173

# Start Vite dev server accessible from host
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]