# 从零开始构建
FROM scratch

# 添加本地 Alpine minirootfs
ADD alpine-minirootfs-3.21.3-x86_64.tar.gz /

# 设置环境变量
ENV POSTGRES_DB=graduation_project \
    POSTGRES_USER=sleet \
    POSTGRES_PASSWORD="Zyz20050922!" \
    POSTGRES_PORT=5432 \
    REDIS_PORT=6379 \
    APP_PORT=8081

# 配置 Alpine 软件源为阿里云
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装 PostgreSQL 和 Redis
RUN apk update && apk add --no-cache \
    postgresql \
    postgresql-client \
    redis \
    bash \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# 创建必要目录
RUN mkdir -p /run/postgresql /var/lib/postgresql/data && \
    chown postgres:postgres /run/postgresql && \
    chown -R postgres:postgres /var/lib/postgresql && \
    chmod 0700 /var/lib/postgresql/data

# 初始化 PostgreSQL
USER postgres
RUN initdb -D /var/lib/postgresql/data
USER root

# 配置 PostgreSQL 允许远程连接
RUN echo "host all all 0.0.0.0/0 md5" >> /var/lib/postgresql/data/pg_hba.conf && \
    echo "listen_addresses='*'" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "port=5432" >> /var/lib/postgresql/data/postgresql.conf

# 创建启动脚本
RUN cat > /start.sh << 'EOF'
#!/bin/bash
set -e

echo "Starting PostgreSQL..."
su postgres -c "postgres -D /var/lib/postgresql/data" &
PG_PID=$!

echo "Waiting for PostgreSQL..."
until pg_isready -U postgres -h localhost; do
  sleep 1
done

echo "Configuring database..."

# 设置 postgres 密码
psql -U postgres -c "ALTER USER postgres WITH PASSWORD '${POSTGRES_PASSWORD}';"

# 创建用户
psql -U postgres -c "CREATE USER ${POSTGRES_USER} WITH PASSWORD '${POSTGRES_PASSWORD}';" 2>/dev/null || true

# 授予用户创建数据库权限
psql -U postgres -c "ALTER USER ${POSTGRES_USER} WITH CREATEDB;" 2>/dev/null || true

# 授予超级用户权限（确保所有权限）
psql -U postgres -c "ALTER USER ${POSTGRES_USER} WITH SUPERUSER;" 2>/dev/null || true

# 创建 graduation_project 数据库
psql -U postgres -c "CREATE DATABASE graduation_project OWNER ${POSTGRES_USER};" 2>/dev/null || true

# 配置数据库权限
psql -U postgres -d graduation_project << 'EOSQL'
-- 授予 schema 权限
GRANT USAGE ON SCHEMA public TO sleet;
GRANT CREATE ON SCHEMA public TO sleet;
GRANT ALL PRIVILEGES ON SCHEMA public TO sleet;

-- 授予数据库所有权限
GRANT ALL PRIVILEGES ON DATABASE graduation_project TO sleet;

-- 设置默认权限
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO sleet;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO sleet;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO sleet;

-- 将数据库所有者改为 sleet
ALTER DATABASE graduation_project OWNER TO sleet;
EOSQL

# 创建 sleet 数据库（备用）
psql -U postgres -c "CREATE DATABASE sleet OWNER ${POSTGRES_USER};" 2>/dev/null || true
psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE sleet TO ${POSTGRES_USER};" 2>/dev/null || true

echo "Databases created:"
psql -U postgres -c "\l"

echo "Starting Redis..."
# 启动 Redis：关闭保护模式，绑定所有接口，无密码
redis-server --daemonize yes \
  --protected-mode no \
  --bind 0.0.0.0

echo "Waiting for Redis..."
until redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG; do
  sleep 1
done

echo "Starting application..."
exec /app/main
EOF

RUN chmod +x /start.sh

# 复制应用程序
COPY ./main /app/main
COPY ./configs/config.yaml /app/configs/config.yaml
RUN chmod +x /app/main

WORKDIR /app

EXPOSE 5432 6379 8081

CMD ["/start.sh"]