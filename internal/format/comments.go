package format

type CommentDialect struct {
	LinePrefix string
	Supported  bool
}

func DialectFor(format string) CommentDialect {
	switch format {
	case "kdl":
		return CommentDialect{LinePrefix: "// ", Supported: true}
	case "toml":
		return CommentDialect{LinePrefix: "# ", Supported: true}
	case "yaml", "yml":
		return CommentDialect{LinePrefix: "# ", Supported: true}
	case "ini":
		return CommentDialect{LinePrefix: "; ", Supported: true}
	case "json", "raw":
		fallthrough
	default:
		return CommentDialect{Supported: false}
	}
}

func RenderHeader(d CommentDialect, lines []string) []byte {
	if !d.Supported || len(lines) == 0 {
		return nil
	}
	out := make([]byte, 0, 256)
	for _, l := range lines {
		out = append(out, []byte(d.LinePrefix+l+"\n")...)
	}
	out = append(out, '\n')
	return out
}
