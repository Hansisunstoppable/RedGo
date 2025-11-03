# RedGo
从零开始，基于 Golang 实现的一个简易 Redis 服务器

## 实现功能
- 网络层：TCP 服务器
- 协议层：RESP 协议解析器
- 存储引擎：内存数据库
- 数据结构：String、List、Hash、Set、ZSet
- 并发安全：Key级别细粒度锁定机制
- 持久化：AOF (Append Only File) 机制
- 集群：一致性哈希

## 快速开始
### 启动服务端
```bash
# 克隆项目
git clone git@github.com:Hansisunstoppable/RedGo.git 
cd RedGo

# 启动单机模式
go run Godis

# 启动集群模式（编辑 redis.conf 设置集群节点）
go run Godis
```
### 客户端连接测试
```bash
# 使用 Redis 官方客户端
redis-cli -h localhost -p 6380

# 测试基本命令
127.0.0.1:6380> SET hello world
OK
127.0.0.1:6380> GET hello
"world"
127.0.0.1:6380> PING
PONG
```
## 性能基准与压力测试
使用 Redis 官方提供的 `redis-benchmar` 来进行性能测试。
### 测试命令参数
| 参数                | 描述               | 示例               |
|---------------------|--------------------|--------------------|
| `-h <hostname>`     | Redis 服务器地址   | `-h localhost`     |
| `-p <port>`         | Redis 服务器端口   | `-p 6380`          |
| `-n <requests>`     | 总请求数           | `-n 100000`        |
| `-c <clients>`      | 并发连接数         | `-c 50`            |
| `-d <size>`         | 数据大小（字节）   | `-d 1024`          |
| `-t <tests>`        | 指定测试命令       | `-t set,get,lpush` |
| `-k <boolean>`      | 保持连接           | `-k 1`             |
| `-r <keyspacelen>`  | 键空间大小         | `-r 100000`        |
| `-P <pipeline>`     | 管道请求数         | `-P 10`            |
| `-q`                | 静默模式，只显示结果 | `-q`             |
| `--csv`             | CSV 格式输出       | `--csv`            |
### 基本测试命令
```bash
# 启动 RedGo 服务器
go run RedGo

# 在另一个终端运行基准测试
redis-benchmark -h localhost -p 6379 -n 100000 -c 50
```
### 测试结果
测试环境：AMD Ryzen 7 7840H, 32GB RAM, Ubuntu 22.04 LTS (WSL 2 on Windows 11)

**关键指标说明：**

- Requests per second (QPS)：每秒处理的请求数，越高越好
- Latency percentiles：延迟百分位数，显示响应时间分布
- 平均延迟：所有请求的平均响应时间
- 吞吐量：服务器的数据处理能力

#### 核心操作性能表现
| 操作类型 | QPS | 平均延迟(ms) | P95延迟(ms) | P99延迟(ms) |
|----------|-----|--------------|-------------|-------------|
| SET      |     |              |             |             |
| GET      |     |              |             |             |
| LPUSH    |     |              |             |             |
| LPOP     |     |              |             |             |
| HSET     |     |              |             |             |
| SADD     |     |              |             |             |
| SPOP     |     |              |             |             |
| ZADD     |     |              |             |             |
#### 范围查询性能表现
| LRANGE操作     | QPS | 平均延迟(ms) | P95延迟(ms) | P99延迟(ms) | 适用场景         |
|----------------|-----|--------------|-------------|-------------|------------------|
| LRANGE_100     |     |              |             |             |小数据量查询|
| LRANGE_300     |     |              |             |             |中等数据量查询 |
| LRANGE_500     |     |              |             |             |大数据量查询|
| LRANGE_600     |     |              |             |             |超大数据量查询 |

