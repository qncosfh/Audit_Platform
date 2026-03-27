# ================== 构建阶段 ==================
FROM mirror.ccs.tencentyun.com/library/golang:1.25-alpine AS builder

# 换源 + 安装 git
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache git

WORKDIR /app

# 先拷贝依赖（利用缓存）
COPY go.mod go.sum ./

# Go 国内加速
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

RUN go mod download

# 再拷贝源码
COPY . .

# 编译（减小体积）
RUN go build -ldflags="-s -w" -o platform cmd/main.go


# ================== 运行阶段 ==================
FROM mirror.ccs.tencentyun.com/library/alpine:3.18

# 🚀 关键优化：换国内源 + 一次性安装
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache \
        ca-certificates \
        tzdata \
        curl \
        git \
        openjdk17

# 时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 拷贝二进制
COPY --from=builder /app/platform .

# 创建目录
RUN mkdir -p /app/sandbox

# 端口
EXPOSE 8080

# 启动
CMD ["./platform"]
