package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dating-app/services/chat/internal/model"
)

const mediaProbeTimeout = 10 * time.Second

// FFprobeInspector определяет контейнер, тип дорожек и длительность на сервере.
type FFprobeInspector struct {
	binary string
}

func NewFFprobeInspector(binary string) *FFprobeInspector {

	if binary == "" {
		binary = "ffprobe"
	}

	return &FFprobeInspector{binary: binary}
}

func (p *FFprobeInspector) Inspect(ctx context.Context, data []byte, mediaType model.ChatMessageType) (*model.MediaInfo, error) {

	file, err := os.CreateTemp("", "chat-media-*")

	if err != nil {
		return nil, fmt.Errorf("create media inspection file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.Write(data); err != nil {
		file.Close()
		return nil, fmt.Errorf("write media inspection file: %w", err)
	}

	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close media inspection file: %w", err)
	}

	probeCtx, cancel := context.WithTimeout(ctx, mediaProbeTimeout)
	defer cancel()

	// Запрещаем сетевые протоколы; проверяем только загруженный файл.
	output, err := exec.CommandContext(probeCtx, p.binary,
		"-v", "error", "-protocol_whitelist", "file", "-format_whitelist", "mov,matroska,webm,ogg,mp3,wav",
		"-show_packets", "-show_entries", "format=format_name,duration:stream=codec_type,duration:stream_tags=DURATION:packet=pts_time,duration_time",
		"-of", "json", file.Name(),
	).Output()

	if err != nil {

		if probeCtx.Err() != nil {
			return nil, probeCtx.Err()
		}

		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) {
			return nil, model.ErrUnsupportedMedia
		}

		return nil, fmt.Errorf("run ffprobe: %w", err)
	}

	return mediaInfoFromProbe(output, mediaType)
}

func mediaInfoFromProbe(data []byte, mediaType model.ChatMessageType) (*model.MediaInfo, error) {

	var probe struct {
		Packets []struct {
			PTS      string `json:"pts_time"`
			Duration string `json:"duration_time"`
		} `json:"packets"`
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type     string `json:"codec_type"`
			Duration string `json:"duration"`
			Tags     struct {
				Duration string `json:"DURATION"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("decode ffprobe response: %w", err)
	}

	audio, video := false, false
	duration, _ := strconv.ParseFloat(probe.Format.Duration, 64)

	for _, stream := range probe.Streams {

		switch stream.Type {

		case "audio":
			audio = true

		case "video":
			video = true

		default:
			return nil, model.ErrUnsupportedMedia
		}

		streamDuration, _ := strconv.ParseFloat(stream.Duration, 64)

		if streamDuration <= 0 && stream.Tags.Duration != "" {
			var hours, minutes int
			var seconds float64

			if n, _ := fmt.Sscanf(stream.Tags.Duration, "%d:%d:%f", &hours, &minutes, &seconds); n == 3 {
				streamDuration = float64(hours*3600+minutes*60) + seconds
			}
		}

		if streamDuration > duration {
			duration = streamDuration
		}
	}

	if mediaType == model.ChatMessageTypeVoice && (!audio || video) ||
		mediaType == model.ChatMessageTypeVideoNote && !video {
		return nil, model.ErrUnsupportedMedia
	}

	// WebM из MediaRecorder может не содержать длительность в заголовке.
	// В этом случае определяем её по временным отметкам пакетов.
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		start, end := math.Inf(1), math.Inf(-1)

		for _, packet := range probe.Packets {
			pts, ptsErr := strconv.ParseFloat(packet.PTS, 64)
			packetDuration, _ := strconv.ParseFloat(packet.Duration, 64)

			if ptsErr != nil || math.IsNaN(pts) || math.IsInf(pts, 0) {
				continue
			}

			if packetDuration < 0 || math.IsNaN(packetDuration) || math.IsInf(packetDuration, 0) {
				packetDuration = 0
			}

			start = math.Min(start, pts)
			end = math.Max(end, pts+packetDuration)
		}

		duration = end - start
	}

	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return nil, model.ErrInvalidMedia
	}

	prefix := "audio/"

	if mediaType == model.ChatMessageTypeVideoNote {
		prefix = "video/"
	} else if mediaType != model.ChatMessageTypeVoice {
		return nil, model.ErrUnsupportedMedia
	}

	formats := "," + probe.Format.Name + ","
	contentType := ""

	switch {

	case strings.Contains(formats, ",webm,"):
		contentType = prefix + "webm"

	case strings.Contains(formats, ",mp4,"):
		contentType = prefix + "mp4"

	case strings.Contains(formats, ",ogg,"):
		contentType = prefix + "ogg"

	case mediaType == model.ChatMessageTypeVoice && strings.Contains(formats, ",mp3,"):
		contentType = "audio/mpeg"

	case mediaType == model.ChatMessageTypeVoice && strings.Contains(formats, ",wav,"):
		contentType = "audio/wav"

	default:
		return nil, model.ErrUnsupportedMedia
	}

	return &model.MediaInfo{ContentType: contentType, DurationSeconds: duration}, nil
}
