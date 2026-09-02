FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm install --package-lock-only --ignore-scripts && npm ci
COPY . .
RUN npm run build

FROM nginx:1.29-alpine
COPY deployments/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 8080
USER nginx
