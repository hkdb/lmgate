package admin

import "sync"

type LogNotifier struct {
	mu   sync.Mutex
	subs map[uint64]chan struct{}
	next uint64
}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{
		subs: make(map[uint64]chan struct{}),
	}
}

func (n *LogNotifier) Subscribe() (uint64, chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	id := n.next
	n.next++
	ch := make(chan struct{}, 1)
	n.subs[id] = ch
	return id, ch
}

func (n *LogNotifier) Unsubscribe(id uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ch, ok := n.subs[id]; ok {
		close(ch)
		delete(n.subs, id)
	}
}

func (n *LogNotifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for id, ch := range n.subs {
		close(ch)
		delete(n.subs, id)
	}
}

func (n *LogNotifier) Notify() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, ch := range n.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
