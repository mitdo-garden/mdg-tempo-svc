package remind

import "strings"

// Message is a short Telegram card: the tempo name, then its tags.
func Message(name string, tags []string) string {
	name = strings.TrimSpace(name)
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		tag = strings.Join(strings.Fields(tag), " ")
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	if name == "" && len(cleaned) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<blockquote>")
	if name != "" {
		b.WriteString("🌿  <b>")
		b.WriteString(escapeHTML(name))
		b.WriteString("</b>")
	}
	if len(cleaned) > 0 {
		if name != "" {
			b.WriteByte('\n')
		}
		b.WriteString("<i>")
		for i, tag := range cleaned {
			if i > 0 {
				b.WriteString("  ·  ")
			}
			b.WriteString(escapeHTML(tag))
		}
		b.WriteString("</i>")
	}
	b.WriteString("</blockquote>")
	return b.String()
}

func escapeHTML(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}
