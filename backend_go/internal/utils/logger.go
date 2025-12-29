package utils

type TxLogger struct {
	Logs []string
}

func NewTxLogger() *TxLogger {
	return &TxLogger{Logs: make([]string, 0, 64)}
}

func (l *TxLogger) Info(msg string) {
	l.Logs = append(l.Logs, "[INFO] "+msg)
}

func (l *TxLogger) SQL(msg string) {
	l.Logs = append(l.Logs, "[SQL] "+msg)
}

func (l *TxLogger) Raw(msg string) {
	l.Logs = append(l.Logs, msg)
}
