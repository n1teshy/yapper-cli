package piper

type ProgressWriter struct {
	Total      int64
	Current    int64
	lastBarLen int
	Hook       func(*ProgressWriter)
}

func (pw *ProgressWriter) Write(chunk []byte) (int, error) {
	length := int64(len(chunk))
	pw.Current += length
	pw.Hook(pw)
	return len(chunk), nil
}
