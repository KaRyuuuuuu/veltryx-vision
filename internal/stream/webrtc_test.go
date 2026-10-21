//go:build x264 && cgo

package stream

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/veltryx/vision/apps/encoder"
	"github.com/veltryx/vision/internal/frames"
)

// This is a real local ICE/DTLS/SRTP exchange using synthetic images, not a hardware test.
func TestTwoPeersReceiveOneEncodedStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	hub := frames.NewHub()
	publish := func(seq uint64) {
		hub.Publish(&frames.Frame{Width: 64, Height: 48, Sequence: seq, Timestamp: time.Now(), Pixels: make([]byte, 64*48)})
	}
	publish(1)
	s, e := New(ctx, hub, encoder.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	go func() {
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		seq := uint64(1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				seq++
				publish(seq)
			}
		}
	}()
	received := make(chan int, 2)
	for i := 0; i < 2; i++ {
		pc, e := webrtc.NewPeerConnection(webrtc.Configuration{})
		if e != nil {
			t.Fatal(e)
		}
		defer pc.Close()
		if _, e = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); e != nil {
			t.Fatal(e)
		}
		pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			if packet, _, err := track.ReadRTP(); err == nil {
				received <- len(packet.Payload)
			}
		})
		offer, e := pc.CreateOffer(nil)
		if e != nil {
			t.Fatal(e)
		}
		gather := webrtc.GatheringCompletePromise(pc)
		if e = pc.SetLocalDescription(offer); e != nil {
			t.Fatal(e)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-gather:
		}
		answer, e := s.Answer(ctx, *pc.LocalDescription())
		if e != nil {
			t.Fatal(e)
		}
		if e = pc.SetRemoteDescription(answer); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		select {
		case size := <-received:
			if size == 0 {
				t.Fatal("empty RTP packet")
			}
		case <-ctx.Done():
			t.Fatal("no H.264 RTP received", ctx.Err())
		}
	}
	if s.Stats().Peers != 2 {
		t.Fatal(s.Stats())
	}
	s.Close()
	if _, e = s.Answer(ctx, webrtc.SessionDescription{}); e == nil {
		t.Fatal("closed stream accepted peer")
	}
}
