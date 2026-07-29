#!/usr/bin/env bash
# 2.0 用：生成 Master / Worker 自签 mTLS 证书
# 1.0 简化版暂不启用，placeholder
set -euo pipefail

OUT_DIR="${1:-./certs}"
mkdir -p "$OUT_DIR"

# 实际生成时用 cfssl 或 step-ca，这里留作后续任务
echo "TODO: mTLS 证书生成脚本（Phase 2 / Task 2.x）"
echo "输出目录：$OUT_DIR"
