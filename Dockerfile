FROM golang:1.22

# 必要に応じて追加パッケージ
RUN apt-get update && apt-get install -y \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

# デフォルトで bash/zsh どちらでもOK
CMD ["bash"]
