#!/bin/bash

# ============ 配置参数 ============
HOST="localhost"
PORT="6379"
REQUESTS=10000
CLIENTS=40
KEY="bench_zset"
ZSET_SIZE=500  # zrange_500 需要 0~499 共 500 个成员

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}正在准备 Redis ZRANGE 压测数据...${NC}"

# ============ 步骤 1：清空并填充 bench_zset ============
echo -e "${GREEN}清空并填充 key: $KEY (共 $ZSET_SIZE 个成员)${NC}"

# 使用 redis-cli 管道批量插入（高效）
(
  echo "DEL $KEY"
  for ((i=0; i<ZSET_SIZE; i++)); do
    echo "ZADD $KEY $i $i"
  done
) | redis-cli -h "$HOST" -p "$PORT" > /dev/null 2>&1

# 检查是否成功
ZSET_COUNT=$(redis-cli -h "$HOST" -p "$PORT" ZCARD "$KEY")
if [ "$ZSET_COUNT" -eq "$ZSET_SIZE" ]; then
  echo -e "${GREEN}数据填充成功！ZSET 成员数: $ZSET_COUNT${NC}"
else
  echo -e "${RED}数据填充失败！当前成员数: $ZSET_COUNT，期望: $ZSET_SIZE${NC}"
  exit 1
fi

# ============ 步骤 2：执行 redis-benchmark ============
echo -e "${YELLOW}开始执行 redis-benchmark 压测...${NC}"

redis-benchmark -h "$HOST" -p "$PORT" -n "$REQUESTS" -c "$CLIENTS" -t zrange_100,zrange_300,zrange_500

# ============ 完成 ============
echo -e "${GREEN}压测完成！${NC}"