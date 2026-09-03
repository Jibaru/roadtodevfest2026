package tts

import (
	"context"
	"encoding/binary"
	"fmt"

	"google.golang.org/genai"

	"github.com/jibaru/rapbattle/internal/battle/domain"
)

const ttsModel = "gemini-2.5-flash-preview-tts"

// Gemini TTS returns raw 16-bit PCM, mono, 24kHz.
const (
	sampleRate    = 24000
	numChannels   = 1
	bitsPerSample = 16
)

// voices gives each battler a recognizable character:
// Puck is upbeat (the Gopher), Kore is firm and cold (NULL PTR).
var voices = map[domain.Battler]string{
	domain.BattlerGopher:  "Puck",
	domain.BattlerNullPtr: "Kore",
}

var deliveries = map[domain.Battler]string{
	domain.BattlerGopher:  "Perform this rap verse with high energy, fast playful flow and total confidence",
	domain.BattlerNullPtr: "Perform this rap verse in a cold, menacing, deadpan rhythmic delivery",
}

// Client synthesizes battle verses with the Gemini TTS models.
// Implements service.Performer.
type Client struct {
	genai *genai.Client
}

func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating genai client: %w", err)
	}
	return &Client{genai: c}, nil
}

// Synthesize returns a browser-playable WAV of the battler performing the verse.
func (c *Client) Synthesize(ctx context.Context, battler domain.Battler, verse string) ([]byte, error) {
	voice, ok := voices[battler]
	if !ok {
		return nil, domain.ErrInvalidBattler
	}

	prompt := fmt.Sprintf("%s:\n\n%s", deliveries[battler], verse)
	cfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: voice},
			},
		},
	}

	result, err := c.genai.Models.GenerateContent(ctx, ttsModel, genai.Text(prompt), cfg)
	if err != nil {
		return nil, fmt.Errorf("tts generate: %w", err)
	}
	if len(result.Candidates) == 0 || result.Candidates[0].Content == nil ||
		len(result.Candidates[0].Content.Parts) == 0 ||
		result.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, fmt.Errorf("tts returned no audio")
	}
	pcm := result.Candidates[0].Content.Parts[0].InlineData.Data
	return pcmToWAV(pcm), nil
}

// pcmToWAV prepends the 44-byte RIFF/WAVE header the browser needs.
func pcmToWAV(pcm []byte) []byte {
	blockAlign := numChannels * bitsPerSample / 8
	byteRate := sampleRate * blockAlign

	buf := make([]byte, 0, 44+len(pcm))
	buf = append(buf, "RIFF"...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(36+len(pcm)))
	buf = append(buf, "WAVE"...)
	buf = append(buf, "fmt "...)
	buf = binary.LittleEndian.AppendUint32(buf, 16) // PCM fmt chunk size
	buf = binary.LittleEndian.AppendUint16(buf, 1)  // audio format: PCM
	buf = binary.LittleEndian.AppendUint16(buf, numChannels)
	buf = binary.LittleEndian.AppendUint32(buf, sampleRate)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(byteRate))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(blockAlign))
	buf = binary.LittleEndian.AppendUint16(buf, bitsPerSample)
	buf = append(buf, "data"...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(pcm)))
	return append(buf, pcm...)
}
