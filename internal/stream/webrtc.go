package stream

import (
	"context"
	"errors"
	"time"

	"github.com/pion/webrtc/v4"
)

var errNoFrame = errors.New("no camera frame available")

func (s *Stream) Answer(ctx context.Context, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	s.mu.Lock()
	if s.closed || len(s.peers) >= 16 {
		s.mu.Unlock()
		return webrtc.SessionDescription{}, errors.New("maximum 16 peers per camera")
	}
	s.mu.Unlock()
	pc, e := webrtc.NewPeerConnection(webrtc.Configuration{})
	if e != nil {
		return webrtc.SessionDescription{}, e
	}
	ok := false
	defer func() {
		if !ok {
			_ = pc.Close()
		}
	}()
	sender, e := pc.AddTrack(s.track)
	if e != nil {
		return webrtc.SessionDescription{}, e
	}
	s.mu.Lock()
	if s.closed || len(s.peers) >= 16 {
		s.mu.Unlock()
		return webrtc.SessionDescription{}, errors.New("stream closed or peer capacity reached")
	}
	s.peers[pc] = struct{}{}
	s.mu.Unlock()
	remove := func() { s.mu.Lock(); delete(s.peers, pc); s.mu.Unlock() }
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			remove()
			if state != webrtc.PeerConnectionStateClosed {
				go pc.Close()
			}
		}
	})
	defer func() {
		if !ok {
			remove()
		}
	}()
	go func() {
		b := make([]byte, 1500)
		for {
			if _, _, e := sender.Read(b); e != nil {
				return
			}
		}
	}()
	if e = pc.SetRemoteDescription(offer); e != nil {
		return webrtc.SessionDescription{}, e
	}
	answer, e := pc.CreateAnswer(nil)
	if e != nil {
		return answer, e
	}
	gather := webrtc.GatheringCompletePromise(pc)
	if e = pc.SetLocalDescription(answer); e != nil {
		return answer, e
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return answer, ctx.Err()
	case <-timer.C:
		return answer, errors.New("ICE gathering timeout")
	case <-gather:
	}
	ok = true
	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		<-timer.C
		if pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
			_ = pc.Close()
		}
	}()
	return *pc.LocalDescription(), nil
}
