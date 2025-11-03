package cluster

// connectionFactory 连接池内对象的创建、销毁逻辑
// 实现了 PooledObjectFactory，在连接池创建时传入，让连接池知道管理什么对象、如何创建与销毁

import (
	"Godis/resp/client"
	"context"
	"errors"

	pool "github.com/jolestar/go-commons-pool/v2"
)

// connectionFactory 工厂方法，规定了连接池应该创建什么对象（client）
type connectionFactory struct {
	Peer string // peer node id
}

// 创建连接池对象, 可以监听 ctx.Done() 通过 ctx 实现超时\取消控制
func (f *connectionFactory) MakeObject(ctx context.Context) (*pool.PooledObject, error) {
	c, err := client.MakeClient(f.Peer)
	if err != nil {
		return nil, err
	}
	c.Start() // 创建后直接启动
	return pool.NewPooledObject(c), nil
}

// DestroyObject 销毁连接池内对象 object
func (f *connectionFactory) DestroyObject(ctx context.Context, object *pool.PooledObject) error {
	c, ok := object.Object.(*client.Client) // 断言：连接池内待销毁的对象一定是 Client 类型
	if !ok {
		return errors.New("invalid connection type")
	}
	c.Close() // 关闭该 client
	return nil
}

// 下面三个方法不必实现

func (f *connectionFactory) ValidateObject(ctx context.Context, object *pool.PooledObject) bool {
	return true
}
func (f *connectionFactory) ActivateObject(ctx context.Context, object *pool.PooledObject) error {
	return nil
}
func (f *connectionFactory) PassivateObject(ctx context.Context, object *pool.PooledObject) error {
	return nil
}
