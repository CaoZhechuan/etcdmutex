# etcdmutex

 etcdmutex 是 Go 中使用 etcd 的分布式锁库。它像sync.Mutex一样易于使用
 
 其实有很多类似的实现都已经过时了，取决于具体"go.etcd.io/etcd/clientv3"官方标注的库，用法很简单，代码也很简单。
 
 #Import
 
 > go get -u github.com/CaoZhechuan/etcdmutex

#Simplest usage
> 争抢锁默认抢三次，每一秒尝试获取一次，若三次未获取到锁，则放弃争抢锁，因此需要对error判断，避免对未获取锁进行解锁操作。

Steps:
1. m, err := etcdmutex.New()
2. m.Lock()
3. Do your business here
4. m.Unlock()

```go
package main

import (
	"log"
	"os"

	"github.com/CaoZhechuan/etcdmutex"
)

func main() {
	m, err := etcdsync.New("/mylock", 10, []string{"http://127.0.0.1:2379"})
	if m == nil || err != nil {
		log.Printf("etcdsync.New failed")
		return
	}
	err = m.Lock()
	if err != nil {
		log.Printf("etcdsync.Lock failed")
	} else {
		log.Printf("etcdsync.Lock OK")
	}

	log.Printf("Get the lock. Do something here.")

	err = m.Unlock()
	if err != nil {
		log.Printf("etcdsync.Unlock failed")
	} else {
		log.Printf("etcdsync.Unlock OK")
	}
}


```