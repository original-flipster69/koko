package agent

import (
	"testing"

	"github.com/original-flipster69/koko/internal/provider"
)

func TestScrubPIIFilter_PreservesSystemMessage(t *testing.T) {
	in := []provider.Msg{
		{Role: provider.System, Content: "system prompt — should NOT be scrubbed"},
	}
	out := ScrubPIIFilter(in)
	if len(out) != 1 {
		t.Fatalf("length mismatch: got %d, want 1", len(out))
	}
	if out[0].Role != provider.System {
		t.Errorf("system role changed: %v", out[0].Role)
	}
	if out[0].Content != in[0].Content {
		t.Errorf("system content modified: %q → %q", in[0].Content, out[0].Content)
	}
}

func TestScrubPIIFilter_PreservesLengthAndOrder(t *testing.T) {
	in := []provider.Msg{
		{Role: provider.System, Content: "sys"},
		{Role: provider.User, Content: "hello"},
		{Role: provider.Assistant, Content: "world"},
		{Role: provider.User, Content: "again"},
	}
	out := ScrubPIIFilter(in)
	if len(out) != len(in) {
		t.Fatalf("length mismatch: got %d, want %d", len(out), len(in))
	}
	for i := range out {
		if out[i].Role != in[i].Role {
			t.Errorf("role mismatch at %d: got %v, want %v", i, out[i].Role, in[i].Role)
		}
	}
}

func TestScrubPIIFilter_DoesNotMutateInputContent(t *testing.T) {
	original := "original content with email foo@bar.com"
	in := []provider.Msg{
		{Role: provider.User, Content: original},
	}
	_ = ScrubPIIFilter(in)
	if in[0].Content != original {
		t.Errorf("input was mutated: %q → %q", original, in[0].Content)
	}
}

func TestScrubPIIFilter_PreservesPlainContent(t *testing.T) {
	in := []provider.Msg{
		{Role: provider.User, Content: "Hello, world! This is plain text with no secrets."},
		{Role: provider.Assistant, Content: "Plain reply."},
	}
	out := ScrubPIIFilter(in)
	for i := range out {
		if out[i].Content != in[i].Content {
			t.Errorf("plain content at %d modified: %q → %q", i, in[i].Content, out[i].Content)
		}
	}
}

func TestScrubPIIFilter_PreservesImages(t *testing.T) {
	img := provider.Img{Mime: "image/png", Data: "base64bytes"}
	in := []provider.Msg{
		{Role: provider.User, Content: "look at this", Imgs: []provider.Img{img}},
	}
	out := ScrubPIIFilter(in)
	if len(out[0].Imgs) != 1 {
		t.Fatalf("expected 1 image preserved, got %d", len(out[0].Imgs))
	}
	if out[0].Imgs[0].Mime != img.Mime || out[0].Imgs[0].Data != img.Data {
		t.Errorf("image fields not preserved: %+v vs %+v", out[0].Imgs[0], img)
	}
}

func TestScrubPIIFilter_EmptyInput(t *testing.T) {
	out := ScrubPIIFilter(nil)
	if len(out) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(out))
	}
	out = ScrubPIIFilter([]provider.Msg{})
	if len(out) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(out))
	}
}

func TestScrubPIIFilter_ReturnsNewSlice(t *testing.T) {
	in := []provider.Msg{
		{Role: provider.User, Content: "hello"},
	}
	out := ScrubPIIFilter(in)
	if &in[0] == &out[0] {
		t.Errorf("expected new slice, got same backing array")
	}
}
