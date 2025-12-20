# Stage 1: Build
FROM node:20 AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci --legacy-peer-deps
COPY . .
RUN npm run build:railway

# Stage 2: Serve
FROM nginx:alpine
COPY --from=build /app/dist/pennywise /usr/share/nginx/html
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Default port for local development (Railway overrides this)
ENV PORT=8080

CMD ["nginx", "-g", "daemon off;"]
