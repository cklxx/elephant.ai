package lark

import (
	"testing"
)

func TestResolveTaskUpdateOptions(t *testing.T) {
	t.Run("all options", func(t *testing.T) {
		preview := "answer"
		errText := "error"
		tokens := 500
		mergeStatus := "merged/success"

		vals := ResolveTaskUpdateOptions([]TaskUpdateOption{
			WithAnswerPreview(preview),
			WithErrorText(errText),
			WithTokensUsed(tokens),
			WithMergeStatus(mergeStatus),
		})

		if vals.AnswerPreview == nil || *vals.AnswerPreview != preview {
			t.Errorf("AnswerPreview = %v, want %q", vals.AnswerPreview, preview)
		}
		if vals.ErrorText == nil || *vals.ErrorText != errText {
			t.Errorf("ErrorText = %v, want %q", vals.ErrorText, errText)
		}
		if vals.TokensUsed == nil || *vals.TokensUsed != tokens {
			t.Errorf("TokensUsed = %v, want %d", vals.TokensUsed, tokens)
		}
		if vals.MergeStatus == nil || *vals.MergeStatus != mergeStatus {
			t.Errorf("MergeStatus = %v, want %q", vals.MergeStatus, mergeStatus)
		}
	})

	t.Run("empty options", func(t *testing.T) {
		vals := ResolveTaskUpdateOptions(nil)

		if vals.AnswerPreview != nil {
			t.Errorf("AnswerPreview = %v, want nil", vals.AnswerPreview)
		}
		if vals.ErrorText != nil {
			t.Errorf("ErrorText = %v, want nil", vals.ErrorText)
		}
		if vals.TokensUsed != nil {
			t.Errorf("TokensUsed = %v, want nil", vals.TokensUsed)
		}
		if vals.MergeStatus != nil {
			t.Errorf("MergeStatus = %v, want nil", vals.MergeStatus)
		}
	})
}
