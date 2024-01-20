package log

import (
	"github.com/stn81/kate/log/encoders/simple"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MustNewCoreWithLevelOnly create core only handle specified level
func MustNewCoreWithLevelOnly(level zapcore.Level, location string, enc zapcore.Encoder) zapcore.Core {
	if !path.IsAbs(location) {
		location = path.Join(app.GetHomeDir(), "log", location)
	}

	_ = os.MkdirAll(path.Dir(location), 0755)

	writer, err := NewWriter(location)
	if err != nil {
		panic(fmt.Errorf("failed to create file sink: %v, %v", location, err))
	}

	levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == level
	})

	return zapcore.NewCore(enc, writer, levelEnabler)
}

// MustNewCoreWithLevelAbove create core handle level >= specified level
func MustNewCoreWithLevelAbove(level zapcore.Level, location string, enc zapcore.Encoder) zapcore.Core {
	if !path.IsAbs(location) {
		location = path.Join(app.GetHomeDir(), "log", location)
	}

	_ = os.MkdirAll(path.Dir(location), 0755)

	writer, err := NewWriter(location)
	if err != nil {
		panic(fmt.Errorf("failed to create file sink: %v, %v", location, err))
	}

	return zapcore.NewCore(enc, writer, level)
}

func init() {
	_ = zap.RegisterEncoder("simple", func(config zapcore.EncoderConfig) (encoder zapcore.Encoder, e error) {
		return simple.NewEncoder(), nil
	})
	_ = zap.RegisterSink("rfile", NewSink)
}
