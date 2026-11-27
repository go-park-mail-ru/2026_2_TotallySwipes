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

const (
	mediaProbeTimeout          = 10 * time.Second
	mediaInspectionFilePattern = "chat-media-*"
	probeFloatBitSize          = 64
	secondsPerHour             = int(time.Hour / time.Second)
	secondsPerMinute           = int(time.Minute / time.Second)
	durationTagFormat          = "%d:%d:%f"
	durationTagFieldCount      = 3
)

const (
	ffprobeBinary                = "ffprobe"
	ffprobeLogLevelFlag          = "-v"
	ffprobeLogLevelError         = "error"
	ffprobeProtocolWhitelistFlag = "-protocol_whitelist"
	ffprobeFileProtocol          = "file"
	ffprobeFormatWhitelistFlag   = "-format_whitelist"
	ffprobeAllowedFormats        = "mov,matroska,webm,ogg,mp3,wav"
	ffprobeShowPacketsFlag       = "-show_packets"
	ffprobeShowEntriesFlag       = "-show_entries"
	ffprobeEntries               = "format=format_name,duration:stream=codec_type,duration:stream_tags=DURATION:packet=pts_time,duration_time"
	ffprobeOutputFormatFlag      = "-of"
	ffprobeJSONFormat            = "json"
)

const (
	streamTypeAudio = "audio"
	streamTypeVideo = "video"
	formatSeparator = ","
	formatWebM      = "webm"
	formatMP4       = "mp4"
	formatOgg       = "ogg"
	formatMP3       = "mp3"
	formatWAV       = "wav"
	audioMIMEPrefix = "audio/"
	videoMIMEPrefix = "video/"
	contentTypeMP3  = "audio/mpeg"
	contentTypeWAV  = "audio/wav"
)

// FFprobeInspector определяет контейнер, тип дорожек и длительность на сервере.
type FFprobeInspector struct {
	binary string
}

func NewFFprobeInspector(binary string) *FFprobeInspector {

	if binary == "" {
		binary = ffprobeBinary
	}

	return &FFprobeInspector{binary: binary}
}

func (p *FFprobeInspector) Inspect(ctx context.Context, data []byte, mediaType model.ChatMessageType) (*model.MediaInfo, error) {

	file, err := os.CreateTemp("", mediaInspectionFilePattern)

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

	output, err := exec.CommandContext(probeCtx, p.binary,
		ffprobeLogLevelFlag, ffprobeLogLevelError,
		ffprobeProtocolWhitelistFlag, ffprobeFileProtocol,
		ffprobeFormatWhitelistFlag, ffprobeAllowedFormats,
		ffprobeShowPacketsFlag,
		ffprobeShowEntriesFlag, ffprobeEntries,
		ffprobeOutputFormatFlag, ffprobeJSONFormat,
		file.Name(),
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

	var probe ffprobeResponse

	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("decode ffprobe response: %w", err)
	}

	audio, video := false, false
	duration, _ := strconv.ParseFloat(probe.Format.Duration, probeFloatBitSize)

	for _, stream := range probe.Streams {

		switch stream.Type {

		case streamTypeAudio:
			audio = true

		case streamTypeVideo:
			video = true

		default:
			return nil, model.ErrUnsupportedMedia
		}

		streamDuration, _ := strconv.ParseFloat(stream.Duration, probeFloatBitSize)

		if streamDuration <= 0 && stream.Tags.Duration != "" {
			var hours, minutes int
			var seconds float64

			if n, _ := fmt.Sscanf(stream.Tags.Duration, durationTagFormat, &hours, &minutes, &seconds); n == durationTagFieldCount {
				streamDuration = float64(hours*secondsPerHour+minutes*secondsPerMinute) + seconds
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
			pts, ptsErr := strconv.ParseFloat(packet.PTS, probeFloatBitSize)
			packetDuration, _ := strconv.ParseFloat(packet.Duration, probeFloatBitSize)

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

	prefix := audioMIMEPrefix

	if mediaType == model.ChatMessageTypeVideoNote {
		prefix = videoMIMEPrefix
	} else if mediaType != model.ChatMessageTypeVoice {
		return nil, model.ErrUnsupportedMedia
	}

	formats := formatSeparator + probe.Format.Name + formatSeparator
	contentType := ""

	switch {

	case strings.Contains(formats, formatSeparator+formatWebM+formatSeparator):
		contentType = prefix + formatWebM

	case strings.Contains(formats, formatSeparator+formatMP4+formatSeparator):
		contentType = prefix + formatMP4

	case strings.Contains(formats, formatSeparator+formatOgg+formatSeparator):
		contentType = prefix + formatOgg

	case mediaType == model.ChatMessageTypeVoice && strings.Contains(formats, formatSeparator+formatMP3+formatSeparator):
		contentType = contentTypeMP3

	case mediaType == model.ChatMessageTypeVoice && strings.Contains(formats, formatSeparator+formatWAV+formatSeparator):
		contentType = contentTypeWAV

	default:
		return nil, model.ErrUnsupportedMedia
	}

	return &model.MediaInfo{ContentType: contentType, DurationSeconds: duration}, nil
}
