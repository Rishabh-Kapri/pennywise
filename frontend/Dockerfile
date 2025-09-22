# Stage 1: Build
FROM node:20 AS build
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build:prod

# Stage 2: Serve
FROM nginx:alpine
COPY --from=build /app/dist/pennywise /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 5000
CMD ["nginx", "-g", "daemon off;"]
