package etcdmutex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"io"
	"io/ioutil"
	"sync"
	"time"
)

type Mutex struct {
	key           string
	mutex         *sync.Mutex
	cancel        context.CancelFunc
	config        clientv3.Config
	client        *clientv3.Client
	lease         clientv3.Lease
	leaseResp     *clientv3.LeaseGrantResponse
	leaseId       clientv3.LeaseID
	leaseRespChan <-chan *clientv3.LeaseKeepAliveResponse
	logger        io.Writer
}

// new一个etcd客户端，
func New(key string, ttl int64, endpoints []string, etcdCert string, etcdCertKey string, etcdCa string) (*Mutex, error) {

	//客户端配置
	var m = &Mutex{
		key:   key,
		mutex: new(sync.Mutex),
	}
	var err error
	var ctx context.Context
	// 创建连接-TLS
	cert, err := tls.LoadX509KeyPair(etcdCert, etcdCertKey)
	if err != nil {
		return nil, fmt.Errorf("cert failed, err:%v", err)
	}

	caData, err := ioutil.ReadFile(etcdCa)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)

	_tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}

	m.config = clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		TLS:         _tlsConfig,
	}

	if m.client, err = clientv3.New(m.config); err != nil {
		return nil, err
	}

	//上锁（创建租约，自动续租）
	m.lease = clientv3.NewLease(m.client)

	//设置一个ctx取消自动续租
	ctx, m.cancel = context.WithCancel(context.TODO())

	//设置10秒租约（过期时间）
	m.leaseResp, err = m.lease.Grant(context.TODO(), ttl)
	m.debug("设置10秒租约（过期时间）")
	if err != nil {
		return nil, err
	}

	//拿到租约id
	m.leaseId = m.leaseResp.ID

	//自动续租（不停地往管道中扔租约信息）
	if m.leaseRespChan, err = m.lease.KeepAlive(ctx, m.leaseId); err != nil {
		return nil, err
	}
	//查看锁状态
	go listenLeaseChan(m)
	return m, nil
}

func (m *Mutex) Lock() error {
	m.mutex.Lock()
	var err error
	kv := clientv3.NewKV(m.client)

	//创建事务
	txn := kv.Txn(context.TODO())
	m.debug("创建事务")
	txn.If(clientv3.Compare(clientv3.CreateRevision(m.key), "=", 0)).
		Then(clientv3.OpPut(m.key, "xxx", clientv3.WithLease(m.leaseId))).
		Else(clientv3.OpGet(m.key)) //否则抢锁失败

	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		//提交事务
		if txtResp, err := txn.Commit(); err != nil {
			return nil
		} else {
			//判断是否抢锁
			m.debug("判断是否抢锁")
			if !txtResp.Succeeded {
				err = fmt.Errorf("锁被占用：%s", string(txtResp.Responses[0].GetResponseRange().Kvs[0].Value))
			}
		}
	}
	return err
}

func (m *Mutex) Unlock() error {
	defer m.mutex.Unlock()
	var err error
	m.cancel() //取消自动续约
	m.debug("取消自动续约")
	for i := 0; i < 3; i++ {
		_, err := m.lease.Revoke(context.TODO(), m.leaseId) //中止续约
		if err != nil {
			err = fmt.Errorf("解锁失败：%s", err)
		} else {
			return nil
		}
	}
	return err
}

func listenLeaseChan(m *Mutex) {
	var (
		leaseKeepResp *clientv3.LeaseKeepAliveResponse
	)

	for {
		select {
		case leaseKeepResp = <-m.leaseRespChan:
			if leaseKeepResp == nil {
				//"租期失效，解锁退出"
				m.debug("租期失效，解锁退出")
				goto END
			} else {
				//	fmt.Sprintf("leaseKeepResp ID: %v", leaseKeepResp.ID)
				m.debug(fmt.Sprintf("leaseKeepResp ID: %v", leaseKeepResp.id))
			}
		}
	}
END:
}

func (m *Mutex) debug(format string, v ...interface{}) {
	if m.logger != nil {
		m.logger.Write([]byte(m.leaseId))
		m.logger.Write([]byte(" "))
		m.logger.Write([]byte(fmt.Sprintf(format, v...)))
		m.logger.Write([]byte("\n"))
	}
}

func (m *Mutex) SetDebugLogger(w io.Writer) {
	m.logger = w
}
