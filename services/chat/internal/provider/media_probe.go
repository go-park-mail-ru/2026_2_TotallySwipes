package provider

type ffprobeResponse struct {
	Packets []ffprobePacket `json:"packets"`
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobePacket struct {
	PTS      string `json:"pts_time"`
	Duration string `json:"duration_time"`
}

type ffprobeFormat struct {
	Name     string `json:"format_name"`
	Duration string `json:"duration"`
}

type ffprobeStream struct {
	Type     string            `json:"codec_type"`
	Duration string            `json:"duration"`
	Tags     ffprobeStreamTags `json:"tags"`
}

type ffprobeStreamTags struct {
	Duration string `json:"DURATION"`
}
