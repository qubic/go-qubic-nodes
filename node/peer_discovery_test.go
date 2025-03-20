package node

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestNoPeerDiscovery_UpdatePeers(t *testing.T) {

	discovery := NoPeerDiscovery{}
	assert.Empty(t, discovery.UpdatePeers([]*Node{{}}, []string{"1.2.3.4", "2.3.4.5"}))

}

func TestPeerList_Contains(t *testing.T) {
	peerList := PeerList{
		originalPeers: []string{"1.2.3.4", "2.3.4.5"},
		newPeers:      []string{"3.4.5.6", "4.5.6.7"},
		mutex:         sync.Mutex{},
	}

	assert.True(t, peerList.contains("1.2.3.4"))
	assert.True(t, peerList.contains("2.3.4.5"))
	assert.True(t, peerList.contains("3.4.5.6"))
	assert.True(t, peerList.contains("4.5.6.7"))
	assert.False(t, peerList.contains("5.6.7.8"))
}

func TestPeerList_AddIfNew(t *testing.T) {
	peerList := PeerList{
		originalPeers: []string{"1.2.3.4", "2.3.4.5"},
		newPeers:      []string{"3.4.5.6", "4.5.6.7"},
		mutex:         sync.Mutex{},
	}

	assert.False(t, peerList.AddIfNew("1.2.3.4"))
	assert.False(t, peerList.AddIfNew("2.3.4.5"))
	assert.False(t, peerList.AddIfNew("3.4.5.6"))
	assert.False(t, peerList.AddIfNew("4.5.6.7"))

	assert.True(t, peerList.AddIfNew("5.6.7.8"))
	assert.False(t, peerList.AddIfNew("5.6.7.8")) // already added
}

func TestPublicPeerDiscovery_UpdatePeers(t *testing.T) {

	createNodeFunc := func(host string) (*Node, error) {
		return createTestNodeWithPeers(host, []string{"1.2.3.4", "5.6.7.8", "6.7.8.9", "7.8.9.0"}), nil // 2 new peers
	}
	discovery := newPublicPeerDiscovery(createNodeFunc)

	discoveredPeers := discovery.UpdatePeers([]*Node{
		createTestNodeWithPeers("1.2.3.4",
			[]string{"2.3.4.5", "3.4.5.6", "4.5.6.7", "5.6.7.8"}), //  3 new peers
	}, []string{"1.2.3.4", "2.3.4.5"})

	assert.Len(t, discoveredPeers, 5)

}
