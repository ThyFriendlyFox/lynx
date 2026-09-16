package chat

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrUnknownProvider reports a provider name that Lynx does not know.
var ErrUnknownProvider = errors.New("this chat provider does not exist")

// AnthropicProvider sends the question and the folder to Claude.
type AnthropicProvider struct {
	client anthropic.Client
	cfg    Config
}

// NewAnthropicProvider returns a provider that uses the Claude API.
func NewAnthropicProvider(cfg Config) *AnthropicProvider {
	return &AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(cfg.APIKey)),
		cfg:    cfg,
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string { return ProviderAnthropic }

// Answer streams the answer, so that the person reads it while it arrives.
// The folder description is cached, so the second question in a session
// costs much less than the first one.
func (p *AnthropicProvider) Answer(ctx context.Context, req Request, w io.Writer) (string, error) {
	messages := make([]anthropic.BetaMessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		block := anthropic.NewBetaTextBlock(m.Text)
		if m.Role == "assistant" {
			messages = append(messages, anthropic.BetaMessageParam{
				Role:    anthropic.BetaMessageParamRoleAssistant,
				Content: []anthropic.BetaContentBlockParamUnion{block},
			})
			continue
		}
		messages = append(messages, anthropic.NewBetaUserMessage(block))
	}

	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(p.cfg.Model),
		MaxTokens: p.cfg.MaxTokens,
		System: []anthropic.BetaTextBlockParam{{
			Text:         req.System,
			CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		}},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffort(p.cfg.Effort),
		},
		Messages:  messages,
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
	}

	stream := p.client.Beta.Messages.NewStreaming(ctx, params)
	var message anthropic.BetaMessage
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return "", err
		}
		if delta, ok := event.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.BetaTextDelta); ok {
				if _, err := io.WriteString(w, text.Text); err != nil {
					return "", err
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", describeError(err)
	}
	if message.StopReason == anthropic.BetaStopReasonRefusal {
		return "", fmt.Errorf("the model declined to answer this question (%s)", message.StopDetails.Category)
	}

	answer := ""
	for _, block := range message.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			answer += text.Text
		}
	}
	return answer, nil
}

// describeError turns an API error into a sentence a person acts on.
func describeError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case 401, 403:
		return errors.New("the API key is not valid; set ANTHROPIC_API_KEY or run \"lynx chat --set-key\"")
	case 404:
		return errors.New("this model does not exist for this API key")
	case 429:
		return errors.New("the API rate limit is reached; wait and ask again")
	default:
		if apiErr.StatusCode >= 500 {
			return errors.New("the API is not available; ask again in a moment")
		}
		return err
	}
}
