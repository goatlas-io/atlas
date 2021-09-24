package envoy

import (
	"context"
	"sync"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/sirupsen/logrus"
)

type Callbacks struct {
	Signal         chan struct{}
	Fetches        int
	Requests       int
	DeltaRequests  int
	DeltaResponses int
	mu             sync.Mutex
	log            *logrus.Entry
}

func (cb *Callbacks) Report() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.log.WithField("fetches", cb.Fetches).WithField("requests", cb.Requests).Info("server callbacks")
}
func (cb *Callbacks) OnStreamOpen(_ context.Context, id int64, typ string) error {
	cb.log.WithField("id", id).WithField("type", typ).Debug("stream open")
	connectedClients.Inc()
	return nil
}
func (cb *Callbacks) OnStreamClosed(id int64) {
	cb.log.WithField("id", id).Debug("stream closed")
	connectedClients.Dec()
}
func (cb *Callbacks) OnDeltaStreamOpen(_ context.Context, id int64, typ string) error {
	cb.log.WithField("id", id).WithField("type", typ).Debug("delta stream open")
	return nil
}
func (cb *Callbacks) OnDeltaStreamClosed(id int64) {
	cb.log.WithField("id", id).Debug("delta stream closed")
}
func (cb *Callbacks) OnStreamRequest(int64, *discovery.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.Requests++
	if cb.Signal != nil {
		close(cb.Signal)
		cb.Signal = nil
	}
	return nil
}
func (cb *Callbacks) OnStreamResponse(int64, *discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {
}
func (cb *Callbacks) OnStreamDeltaResponse(id int64, req *discovery.DeltaDiscoveryRequest, res *discovery.DeltaDiscoveryResponse) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaResponses++
}
func (cb *Callbacks) OnStreamDeltaRequest(id int64, req *discovery.DeltaDiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.DeltaRequests++
	if cb.Signal != nil {
		close(cb.Signal)
		cb.Signal = nil
	}

	return nil
}
func (cb *Callbacks) OnFetchRequest(_ context.Context, req *discovery.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.Fetches++
	if cb.Signal != nil {
		close(cb.Signal)
		cb.Signal = nil
	}
	return nil
}
func (cb *Callbacks) OnFetchResponse(*discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {}
