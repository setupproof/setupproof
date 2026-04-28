package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverHTMLCommentMarker(t *testing.T) {
	input := []byte("# Setup\n\n<!-- setupproof id=quickstart timeout=120s -->\n\n<!-- markdownlint-disable-next-line -->\n```sh\nnpm install\nnpm test\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.File != "README.md" {
		t.Fatalf("file = %q", block.File)
	}
	if block.Line != 6 {
		t.Fatalf("line = %d", block.Line)
	}
	if block.MarkerLine != 3 {
		t.Fatalf("marker line = %d", block.MarkerLine)
	}
	if block.Language != "sh" || block.Shell != "sh" {
		t.Fatalf("language/shell = %q/%q", block.Language, block.Shell)
	}
	if block.MarkerForm != MarkerFormHTMLComment {
		t.Fatalf("marker form = %q", block.MarkerForm)
	}
	if block.Metadata["id"] != "quickstart" || block.Metadata["timeout"] != "120s" {
		t.Fatalf("metadata = %#v", block.Metadata)
	}
	if block.Text != "npm install\nnpm test\n" {
		t.Fatalf("text = %q", block.Text)
	}
}

func TestDiscoverInfoStringMarker(t *testing.T) {
	input := []byte("```bash setupproof id=build strict=false\nmake build\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.Line != 1 || block.MarkerLine != 1 {
		t.Fatalf("line/marker line = %d/%d", block.Line, block.MarkerLine)
	}
	if block.Language != "bash" || block.Shell != "bash" {
		t.Fatalf("language/shell = %q/%q", block.Language, block.Shell)
	}
	if block.MarkerForm != MarkerFormInfoString {
		t.Fatalf("marker form = %q", block.MarkerForm)
	}
	if block.Metadata["id"] != "build" || block.Metadata["strict"] != "false" {
		t.Fatalf("metadata = %#v", block.Metadata)
	}
}

func TestInfoStringMarkerMustFollowLanguage(t *testing.T) {
	input := []byte("```sh hl_lines=1 setupproof id=late\nnpm test\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 0 {
		t.Fatalf("expected late marker to be ignored, got %#v", blocks)
	}
}

func TestMarkerMetadataAllowsQuotedWhitespace(t *testing.T) {
	input := []byte("<!-- setupproof id=\"comment id\" -->\n```sh\ntrue\n```\n\n```bash setupproof id='info id' timeout=\"120s\"\ntrue\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Metadata["id"] != "comment id" {
		t.Fatalf("comment metadata = %#v", blocks[0].Metadata)
	}
	if blocks[1].Metadata["id"] != "info id" || blocks[1].Metadata["timeout"] != "120s" {
		t.Fatalf("info metadata = %#v", blocks[1].Metadata)
	}
}

func TestDiscoverShellAlias(t *testing.T) {
	input := []byte("```shell setupproof id=alias\nprintf ok\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Language != "shell" || blocks[0].Shell != "sh" {
		t.Fatalf("language/shell = %q/%q", blocks[0].Language, blocks[0].Shell)
	}
}

func TestDiscoverIgnoresUnmarkedAndReportsUnsupportedMarkedBlocks(t *testing.T) {
	input := []byte("```sh\nnpm install\n```\n\n```python setupproof id=script\nprint('no')\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 marked unsupported block, got %#v", blocks)
	}
	if blocks[0].Language != "python" || blocks[0].Shell != "" {
		t.Fatalf("unsupported block = %#v", blocks[0])
	}
}

func TestHTMLCommentMarkerCanceledByMarkdown(t *testing.T) {
	input := []byte("<!-- setupproof id=lost -->\nThis paragraph cancels the marker.\n```sh\nnpm test\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 0 {
		t.Fatalf("expected no blocks, got %#v", blocks)
	}
}

func TestHTMLCommentMarkerAppliesOnlyToNextFence(t *testing.T) {
	input := []byte("<!-- setupproof id=first -->\n```python\nprint('ignored')\n```\n\n```sh\nnpm test\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 1 {
		t.Fatalf("expected unsupported marked block, got %#v", blocks)
	}
	if blocks[0].Language != "python" || blocks[0].Shell != "" || blocks[0].Metadata["id"] != "first" {
		t.Fatalf("unsupported block = %#v", blocks[0])
	}
}

func TestDiscoverInterleavedMarkerForms(t *testing.T) {
	input := []byte("<!-- setupproof id=comment -->\n```sh\necho comment\n```\n\n```bash setupproof id=info\necho info\n```\n")

	blocks := Discover("README.md", input)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Metadata["id"] != "comment" || blocks[0].MarkerForm != MarkerFormHTMLComment {
		t.Fatalf("first block = %#v", blocks[0])
	}
	if blocks[1].Metadata["id"] != "info" || blocks[1].MarkerForm != MarkerFormInfoString {
		t.Fatalf("second block = %#v", blocks[1])
	}
}

func TestRunmeStyleMetadataDoesNotSelectBlocks(t *testing.T) {
	path := filepath.Join("testdata", "runme-style.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	blocks := Discover(path, contents)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Metadata["id"] != "marked" {
		t.Fatalf("selected wrong block: %#v", blocks[0])
	}
}

func FuzzDiscoverDoesNotPanic(f *testing.F) {
	f.Add([]byte("```sh setupproof id=quickstart\ntrue\n```\n"))
	f.Add([]byte("<!-- setupproof id=quickstart -->\n```bash\nmake test\n```\n"))
	f.Fuzz(func(t *testing.T, contents []byte) {
		_ = Discover("README.md", contents)
		_ = Candidates("README.md", contents)
	})
}
