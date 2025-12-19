FROM node:18-alpine

# Set the working directory
WORKDIR /app

# Copy package.json
COPY package.json ./

# Install dependencies
RUN npm install

# Copy the rest of the application code
COPY . .

# Expose the port the Vite dev server runs on
EXPOSE 5173

# Start Vite dev server accessible from host
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]