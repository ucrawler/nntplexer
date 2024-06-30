package main

import (
	"log"
)

type ClusterDelegate struct{}

func (ClusterDelegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (ClusterDelegate) NotifyMsg(msg []byte) {
	log.Printf("[cluster] NotifyMsg: %s\n", string(msg))
}

func (ClusterDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return [][]byte{}
}

func (ClusterDelegate) LocalState(join bool) []byte {
	return []byte{}
}

func (ClusterDelegate) MergeRemoteState(buf []byte, join bool) {
}
