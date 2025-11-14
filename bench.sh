#!/bin/bash
redis-benchmark \
  -h localhost -p 6379 \
  -n 50000 -c 60 -r 50 \
  -t set,get,lpush,lpop,hset,sadd,spop,zadd \
  -e --csv \
| awk -F',' '
  BEGIN {
    print "命令\t\tQPS\t\t平均延迟(ms)\tP95延迟(ms)\tP99延迟(ms)"
    print "----\t\t---\t\t----------\t----------\t----------"
  }
  $1 ~ /^(SET|GET|LPUSH|LPOP|HSET|SADD|SPOP|ZADD)$/ {
    printf "%-8s\t%-12s\t%-12s\t%-12s\t%-12s\n", $1, $2, $3, $4, $5
  }
'