package services

import (
	"strings"
	"testing"
)

func TestRenderConfirm(t *testing.T) {
	html, err := RenderConfirm(ConfirmEmailData{
		Link: "https://arkana.test/subscribe/confirm?sid=1&token=abc",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Error("expected rendered HTML to include the base64-embedded header logo image")
	}
	if !strings.Contains(html, "https://arkana.test/subscribe/confirm?sid=1") {
		t.Error("expected rendered HTML to include the confirm link")
	}
	if !strings.Contains(html, "token=abc") {
		t.Error("expected rendered HTML to include the confirm token")
	}
	if strings.Contains(html, "{{") {
		t.Error("expected no unrendered template syntax in the output")
	}
}

func TestRenderBroadcast(t *testing.T) {
	html, err := RenderBroadcast(BroadcastEmailData{
		PostTitle:       "Hashing 101",
		PostURL:         "https://arkana.test/cryptography-101/hashing",
		UnsubscribeLink: "https://arkana.test/unsubscribe?sid=1&token=xyz",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "Hashing 101") {
		t.Error("expected rendered HTML to include the post title")
	}
	if !strings.Contains(html, `href="https://arkana.test/cryptography-101/hashing"`) {
		t.Error("expected rendered HTML to include the post link")
	}
	if !strings.Contains(html, "https://arkana.test/unsubscribe?sid=1") {
		t.Error("expected rendered HTML to include the unsubscribe link")
	}
	if !strings.Contains(html, "token=xyz") {
		t.Error("expected rendered HTML to include the unsubscribe token")
	}
	if !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Error("expected rendered HTML to include the base64-embedded header logo image")
	}
	if strings.Contains(html, "{{") {
		t.Error("expected no unrendered template syntax in the output")
	}
}
