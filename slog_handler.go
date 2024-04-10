package coralogix

import (
	"context"
	"log/slog"
	"runtime"
)

type CoralogixHandler struct {
	cxLogger *CoralogixLogger

	opts        slog.HandlerOptions
	defaultData map[string]interface{}
}

type source struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type logMessage struct {
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
	Source  source         `json:"source,omitempty"`
}

func NewCoralogixHandler(privateKey, applicationName, subsystemName string, opts *slog.HandlerOptions) *CoralogixHandler {
	logger := NewCoralogixLogger(
		privateKey,
		applicationName,
		subsystemName,
	)

	return &CoralogixHandler{
		cxLogger: logger,
		opts:     *opts,
	}
}

func (h *CoralogixHandler) cloneData() map[string]interface{} {
	clone := map[string]interface{}{}
	for k, v := range h.defaultData {
		clone[k] = v
	}

	return clone
}

// Handle handles the provided log record.
func (h *CoralogixHandler) Handle(ctx context.Context, r slog.Record) error {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()

	log := logMessage{
		Message: r.Message,
		Data:    h.cloneData(),
	}

	if h.opts.AddSource {
		log.Source = source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}
	}

	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			attrToMap(log.Data, a)
			return true
		})
	}

	category := ""
	if v, ok := log.Data["Category"]; ok {
		category = v.(string)
		delete(log.Data, "Category")
	}

	className := ""
	if v, ok := log.Data["ClassName"]; ok {
		className = v.(string)
		delete(log.Data, "ClassName")
	}

	threadId := ""
	if v, ok := log.Data["ThreadId"]; ok {
		threadId = v.(string)
		delete(log.Data, "ThreadId")
	}

	h.cxLogger.Log(levelSlogToCoralogix(r.Level), log, category, className, f.Function, threadId)
	return nil
}

// WithAttrs returns a new Coralogix whose attributes consists of handler's attributes followed by attrs.
func (h *CoralogixHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	data := h.cloneData()
	for _, attr := range attrs {
		attrToMap(data, attr)
	}

	return &CoralogixHandler{
		cxLogger: h.cxLogger,
		opts:     h.opts,

		defaultData: data,
	}
}

// WithGroup returns a new Coralogix with a group, provided the group's name.
func (h *CoralogixHandler) WithGroup(name string) slog.Handler {
	// not supported yet
	return h
}

// Enabled reports whether the logger emits log records at the given context and level.
// Note: We handover the decision down to the next handler.
func (h *CoralogixHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *CoralogixHandler) Stop() {
	h.cxLogger.Destroy()
}

func attrToMap(m map[string]any, a slog.Attr) {
	switch v := a.Value.Any().(type) {
	case error:
		m[a.Key] = v.Error()
	case []slog.Attr:
		m2 := map[string]any{}
		for _, a2 := range v {
			attrToMap(m2, a2)
			m[a.Key] = m2
		}
	default:
		m[a.Key] = a.Value.Any()
	}
}

func levelSlogToCoralogix(level slog.Level) uint {
	switch level {
	case slog.LevelDebug:
		return Level.DEBUG
	case slog.LevelInfo:
		return Level.INFO
	case slog.LevelWarn:
		return Level.WARNING
	case slog.LevelError:
		return Level.ERROR
	default:
		return uint(level)
	}
}
