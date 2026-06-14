FROM difyz9/ytb2bili-base

# 默认跟随 GitHub 最新 Release，也允许在构建时手动覆盖
ARG VERSION=latest

LABEL maintainer="ZhantaoLi" \
      image="zhantaoli/ytb2bili" \
    version="${VERSION}"

# Docker 自动设置目标架构 (amd64 / arm64)
ARG TARGETARCH

WORKDIR /app

# 从 GitHub Releases 下载对应架构的压缩包并解压
RUN ARCHIVE="ytb2bili-linux-${TARGETARCH}.tar.gz" \
    && if [ "${VERSION}" = "latest" ]; then \
       URL="https://github.com/ZhantaoLi/ytb2bili/releases/latest/download/${ARCHIVE}"; \
     else \
       URL="https://github.com/ZhantaoLi/ytb2bili/releases/download/${VERSION}/${ARCHIVE}"; \
     fi \
    && echo "Downloading ${URL}" \
    && curl -fsSL "${URL}" -o "/tmp/${ARCHIVE}" \
    && tar -xzf "/tmp/${ARCHIVE}" -C /tmp \
    && mv "/tmp/ytb2bili-linux-${TARGETARCH}/ytb2bili-linux-${TARGETARCH}" ./ytb2bili \
    && rm -rf "/tmp/${ARCHIVE}" "/tmp/ytb2bili-linux-${TARGETARCH}" \
    && chmod +x ./ytb2bili

# 创建数据目录
RUN mkdir -p ./logs ./downloads

EXPOSE 8096

CMD ["./ytb2bili"]
