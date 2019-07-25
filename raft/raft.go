package raft

import (
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"simple_raft/tools"
	"sync"
	"time"
)

var (
	// 创建存储leader的对象
	// 最初任期为 0， -1代表没有编号
	leader = Leader{0, -1}
)

// Leader struct
type Leader struct {
	// 任期
	Term int
	// leader id
	LeaderID int
}

// Raft - 声明 Raft 节点类型
type Raft struct {
	mu               sync.Mutex // lock
	id               int        // id
	currentTerm      int        // 当前任期
	votedFor         int        // 为哪个节点投票
	state            int        // 当前节点状态 0-foller 1-candidate 2-leader
	lasteMessageTime int64      // 发送最后一条消息的时间
	currentLeader    int        // 当前 leader

	// channels
	message    chan bool // 消息通道
	electCh    chan bool // 选举通道
	heartBeat  chan bool // 心跳通道
	hearbeatRe chan bool // 心跳信号
	timeout    int       // 超时时间
}

// Make return a new Raft Node
func Make(id int) *Raft {
	rf := &Raft{}

	// 编号
	rf.id = id

	// 给谁都不投
	rf.votedFor = -1

	// init
	rf.setTerm(0)
	rf.currentLeader = -1
	rf.state = 0
	rf.timeout = 0

	// channel
	rf.electCh = make(chan bool)
	rf.message = make(chan bool)
	rf.heartBeat = make(chan bool)
	rf.hearbeatRe = make(chan bool)

	// random
	rand.Seed(time.Now().UnixNano())

	// 选举
	go rf.election()

	// 心跳检查
	go rf.sendLeaderHeartBeat()

	return rf
}

// 设置选举节点
func (rf *Raft) election() {
	// 设置标签
	var result bool

	// 循环投票
	for {
		// timeout
		timeout := tools.RandRange(150, 300)
		// 最后一次投票的时间
		rf.lasteMessageTime = tools.Millisecond()

		select {
		case <-time.After(time.Duration(timeout) * time.Millisecond):
			fmt.Println("当前节点状态(0-foller 1-candidate 2-leader):", rf.state)
		}

		result = false
		// 选leader， 如果选出leader，停止循环，result设置为true
		for !result {
			// 选择谁为leader
			result = rf.electionOneRand(&leader)
		}
	}
}

// 选leader
func (rf *Raft) electionOneRand(leader *Leader) bool {
	// 超时时间
	var timeout int64
	timeout = 100

	//投票数量
	var vote int

	// 当前是否开始心跳信息的方法
	var triggerHeartbeat bool

	// 当前时间戳对应的毫秒
	last := tools.Millisecond()

	// 定义返回值
	success := false

	// 1. 首先要成为 candidate 状态
	rf.mu.Lock()
	rf.becomeCandidate()
	rf.mu.Unlock()

	// 开始选举
	fmt.Println("start electing leader")
	for {

		// 遍历所有的节点投票
		for i := 0; i < tools.RaftCount; i++ {
			// 遍历到不是自己则进行投票
			if i != rf.id {
				// 其他节点，拉票
				go func() {
					// 其他节点没有领导
					if leader.LeaderID < 0 {
						rf.electCh <- true
					}
				}()
			}
		}

		vote = 0
		triggerHeartbeat = false
		// 遍历所有节点进行选举
		for i := 0; i < tools.RaftCount; i++ {
			// 计算投票数量
			select {
			case ok := <-rf.electCh:
				if ok {
					// 投票数+1
					vote++
					// 大于总票数一半, 返回success
					success = vote > tools.RaftCount/2
					// 成领导的状态
					// 如果票数大于一半，且未发出心跳的信号
					if success && !triggerHeartbeat {
						// 选举成功
						// 发送心跳信号
						triggerHeartbeat = true

						rf.mu.Lock()
						// 真正的成为leader
						rf.becomeLeader()
						rf.mu.Unlock()

						// 由leader向其他节点发送心跳信号
						// 心跳信号的通道
						rf.heartBeat <- true
						fmt.Println(rf.id, "号节点成为了leader")
						fmt.Println("leader发送心跳信号")
					}
				}
			}
		}

		// 间隔时间小于100毫秒
		// 若不超时，且票数大于一半，且当前有leader
		if (timeout+last < tools.Millisecond()) || (vote >= tools.RaftCount/2 || rf.currentLeader > -1) {
			break
		} else {
			// 没有选出leader
			select {
			case <-time.After(time.Duration(10) + time.Microsecond):
			}
		}
	}
	return success
}

// 设置发送心跳的方法
func (rf *Raft) sendLeaderHeartBeat() {
	for {
		select {
		case <-rf.heartBeat:
			// 给 leader 返回确认信号
			rf.sendAppendEntriesImpl()
		}
	}
}

// 返回给leader的确认信号
func (rf *Raft) sendAppendEntriesImpl() {
	// 判断当前是否是leader节点
	if rf.currentLeader == rf.id {
		// 确认信号个数
		successCount := 0

		// 设置返回确认信号的子节点
		for i := 0; i < tools.RaftCount; i++ {
			if i != rf.id {
				go func() {
					rp, err := rpc.DialHTTP("tcp", "127.0.0.1:8080")
					if err != nil {
						log.Fatal(err)
					}

					// 接收服务端发来的消息
					ok := false
					err = rp.Call("Raft.Communication", Params{"hello"}, &ok)
					if err != nil {
						log.Fatal(err)
					}
					if ok {
						rf.hearbeatRe <- true
					}
				}()
			}
		}

		// 计算返回确认号的子节点，若子节点个数>raftCount/2，则校验成功
		for i := 0; i < tools.RaftCount; i++ {
			select {
			case ok := <-rf.hearbeatRe:
				if ok {
					// 记录返回确认信号的子节点个数
					successCount++
					if successCount > tools.RaftCount/2 {
						fmt.Println("投票选举成功，校验心跳成功")
						fmt.Println("程序结束")
					}
				}
			}
		}
	}

}

// Communication 等待客户端的消息
func (rf *Raft) Communication(p Params, a *bool) error {
	fmt.Println(p.Msg)
	*a = true
	return nil
}

// 修改节点为leader状态
func (rf *Raft) becomeLeader() {
	// 节点状态变为2，代表leader
	rf.state = 2
	// 把自己设置为 leader
	rf.currentLeader = rf.id
}

// 修改节点为candidate状态
func (rf *Raft) becomeCandidate() {
	// 修改节点状态
	rf.state = 1

	// 节点任期+1
	rf.setTerm(rf.currentTerm + 1)

	// 设置为哪个节点投票（给自己投票）
	rf.votedFor = rf.id

	// 当前没有领导
	rf.currentLeader = -1
}

// 设置任期
func (rf *Raft) setTerm(term int) {
	rf.currentTerm = term
}
