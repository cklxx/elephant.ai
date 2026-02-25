package llm

import (
	"reflect"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestIsImageAttachment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		att         ports.Attachment
		placeholder string
		want        bool
	}{
		{
			name:        "media type png",
			att:         ports.Attachment{MediaType: "image/png"},
			placeholder: "file.bin",
			want:        true,
		},
		{
			name:        "media type jpeg",
			att:         ports.Attachment{MediaType: "image/jpeg"},
			placeholder: "file.bin",
			want:        true,
		},
		{
			name:        "media type uppercase",
			att:         ports.Attachment{MediaType: "IMAGE/PNG"},
			placeholder: "file.bin",
			want:        true,
		},
		{
			name:        "non image media type",
			att:         ports.Attachment{MediaType: "application/pdf"},
			placeholder: "file.bin",
			want:        false,
		},
		{
			name:        "placeholder extension fallback image",
			att:         ports.Attachment{},
			placeholder: "photo.jpg",
			want:        true,
		},
		{
			name:        "placeholder extension fallback non image",
			att:         ports.Attachment{},
			placeholder: "doc.pdf",
			want:        false,
		},
		{
			name:        "attachment name fallback",
			att:         ports.Attachment{Name: "screenshot.png"},
			placeholder: "",
			want:        true,
		},
		{
			name:        "both media type and names empty",
			att:         ports.Attachment{},
			placeholder: "",
			want:        false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ports.IsImageAttachment(tc.att, tc.placeholder)
			if got != tc.want {
				t.Fatalf("IsImageAttachment(%+v, %q) = %v, want %v", tc.att, tc.placeholder, got, tc.want)
			}
		})
	}
}

func TestOrderedImageAttachments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		attachments map[string]ports.Attachment
		want        []string
		wantNil     bool
	}{
		{
			name:        "empty attachments returns nil",
			content:     "hello",
			attachments: map[string]ports.Attachment{},
			wantNil:     true,
		},
		{
			name:    "no placeholders returns sorted images",
			content: "hello world",
			attachments: map[string]ports.Attachment{
				"imgB": {MediaType: "image/png", Name: "imgB.png"},
				"imgA": {MediaType: "image/jpeg", Name: "imgA.jpg"},
			},
			want: []string{"imgA", "imgB"},
		},
		{
			name:    "referenced placeholder appears first",
			content: "Look [img1]",
			attachments: map[string]ports.Attachment{
				"img2": {MediaType: "image/png", Name: "img2.png"},
				"img1": {MediaType: "image/png", Name: "img1.png"},
			},
			want: []string{"img1", "img2"},
		},
		{
			name:    "duplicate placeholder appears once",
			content: "[img1] [img1]",
			attachments: map[string]ports.Attachment{
				"img1": {MediaType: "image/png", Name: "img1.png"},
			},
			want: []string{"img1"},
		},
		{
			name:    "non image attachment skipped",
			content: "attach [doc]",
			attachments: map[string]ports.Attachment{
				"doc": {MediaType: "application/pdf", Name: "doc.pdf"},
				"img": {MediaType: "image/png", Name: "img.png"},
			},
			want: []string{"img"},
		},
		{
			name:    "case insensitive placeholder match",
			content: "see [IMG1]",
			attachments: map[string]ports.Attachment{
				"img2": {MediaType: "image/png", Name: "img2.png"},
				"img1": {MediaType: "image/png", Name: "img1.png"},
			},
			want: []string{"img1", "img2"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := orderedImageAttachments(tc.content, tc.attachments)

			if tc.wantNil {
				if got != nil {
					t.Fatalf("orderedImageAttachments() = %#v, want nil", got)
				}
				return
			}

			gotPlaceholders := descriptorPlaceholders(got)
			if !reflect.DeepEqual(gotPlaceholders, tc.want) {
				t.Fatalf("orderedImageAttachments() placeholders = %v, want %v", gotPlaceholders, tc.want)
			}
		})
	}
}

func TestEmbedAttachmentImages(t *testing.T) {
	t.Parallel()

	t.Run("inline placeholder emits text segments and inline image", func(t *testing.T) {
		t.Parallel()

		content := "Hello [pic] world"
		attachments := map[string]ports.Attachment{
			"pic": {MediaType: "image/png", Name: "pic.png"},
		}

		var texts []string
		var inlineKeys []string
		var trailingKeys []string

		embedAttachmentImages(content, attachments,
			func(text string) {
				texts = append(texts, text)
			},
			func(_ ports.Attachment, key string) bool {
				inlineKeys = append(inlineKeys, key)
				return true
			},
			func(_ ports.Attachment, key string) bool {
				trailingKeys = append(trailingKeys, key)
				return true
			},
		)

		if !reflect.DeepEqual(texts, []string{"Hello ", "[pic]", " world"}) {
			t.Fatalf("appendText calls = %v, want %v", texts, []string{"Hello ", "[pic]", " world"})
		}
		if !reflect.DeepEqual(inlineKeys, []string{"pic"}) {
			t.Fatalf("inline keys = %v, want %v", inlineKeys, []string{"pic"})
		}
		if len(trailingKeys) != 0 {
			t.Fatalf("trailing keys = %v, want none", trailingKeys)
		}
	})

	t.Run("no placeholders sends image as trailing", func(t *testing.T) {
		t.Parallel()

		content := "Hello world"
		attachments := map[string]ports.Attachment{
			"pic": {MediaType: "image/png", Name: "pic.png"},
		}

		var texts []string
		var inlineKeys []string
		var trailingKeys []string

		embedAttachmentImages(content, attachments,
			func(text string) {
				texts = append(texts, text)
			},
			func(_ ports.Attachment, key string) bool {
				inlineKeys = append(inlineKeys, key)
				return true
			},
			func(_ ports.Attachment, key string) bool {
				trailingKeys = append(trailingKeys, key)
				return true
			},
		)

		if !reflect.DeepEqual(texts, []string{"Hello world"}) {
			t.Fatalf("appendText calls = %v, want %v", texts, []string{"Hello world"})
		}
		if len(inlineKeys) != 0 {
			t.Fatalf("inline keys = %v, want none", inlineKeys)
		}
		if !reflect.DeepEqual(trailingKeys, []string{"pic"}) {
			t.Fatalf("trailing keys = %v, want %v", trailingKeys, []string{"pic"})
		}
	})

	t.Run("inline used image is not repeated in trailing", func(t *testing.T) {
		t.Parallel()

		content := "[pic]"
		attachments := map[string]ports.Attachment{
			"pic":   {MediaType: "image/png", Name: "pic.png"},
			"other": {MediaType: "image/png", Name: "other.png"},
		}

		var inlineKeys []string
		var trailingKeys []string

		embedAttachmentImages(content, attachments,
			func(string) {},
			func(_ ports.Attachment, key string) bool {
				inlineKeys = append(inlineKeys, key)
				return true
			},
			func(_ ports.Attachment, key string) bool {
				trailingKeys = append(trailingKeys, key)
				return true
			},
		)

		if !reflect.DeepEqual(inlineKeys, []string{"pic"}) {
			t.Fatalf("inline keys = %v, want %v", inlineKeys, []string{"pic"})
		}
		if !reflect.DeepEqual(trailingKeys, []string{"other"}) {
			t.Fatalf("trailing keys = %v, want %v", trailingKeys, []string{"other"})
		}
	})
}

func descriptorPlaceholders(items []attachmentDescriptor) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Placeholder)
	}
	return out
}
