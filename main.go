package main

import (
	"log"
	"net/http"
	"net/rpc"
	"simple_raft/raft"
	"simple_raft/tools"
)

func main() {

	// 创建三个节点，最初都是 foller 状态
	// 如果出现 candidate 状态的节点，则开始投票
	// 产生leader

	// 创建三节点
	for i := 0; i < tools.RaftCount; i++ {
		// 定义 Make() 创建节点
		raft.Make(i)
	}

	// 对raft结构体实现rpc注册
	rpc.Register(new(raft.Raft))
	rpc.HandleHTTP()
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}

	for {
	}
}
