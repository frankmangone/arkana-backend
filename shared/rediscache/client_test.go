package rediscache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestNewClient(t *testing.T) {
	t.Run("connects successfully to a reachable address", func(t *testing.T) {
		mr := miniredis.RunT(t)

		client, err := NewClient(mr.Addr())
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
	})

	t.Run("returns an error for an unreachable address", func(t *testing.T) {
		_, err := NewClient("127.0.0.1:1")
		if err == nil {
			t.Fatal("expected an error connecting to an unreachable address")
		}
	})
}
