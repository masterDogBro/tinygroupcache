package singelefight

import "sync"

// call 进行中或已经结束的缓存请求
type call struct {
	wg  sync.WaitGroup // WaitGroup并发等待组锁，避免重入
	val interface{}
	err error
}

// CallMap singelefight的主要结构，负责管理不同key的call
type CallMap struct {
	mu sync.Mutex
	m  map[string]*call
}

func (cm *CallMap) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	cm.mu.Lock()
	// CallMap没有New函数，采用延迟初始化
	if cm.m == nil {
		cm.m = make(map[string]*call)
	}

	// 当cm.m[key]已经存在时，意味着已经有对相同key的call请求完成了步骤（1），但还没有完成步骤（2）
	// 此时相同的请求再次到达，只需等待之前相同key的call完成，并返回其结果
	if c, ok := cm.m[key]; ok {
		cm.mu.Unlock()
		c.wg.Wait() // 等待，直至WaitGroup锁为0
		return c.val, c.err
	}

	// 当cm.m[key]不存在时，（1）创建新call并将其加入CallMap，并将WaitGroup锁+1
	c := new(call)
	c.wg.Add(1)
	cm.m[key] = c
	cm.mu.Unlock()

	// （2）调用fn函数，发起请求。再调用完成后，将WaitGroup锁-1
	c.val, c.err = fn()
	c.wg.Done()

	// （3）更新CallMap，删除已完成的call
	cm.mu.Lock()
	delete(cm.m, key)
	cm.mu.Unlock()

	// （4）返回结果
	return c.val, c.err

}
