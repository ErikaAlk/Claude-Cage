package workbench

import (
	"net/http"

	"github.com/sylvinus/agent-vm/internal/mobilelink"
)

func (s *Server) mobileTranscript(w http.ResponseWriter, r *http.Request) {
	view, _, ok := s.authorizedMobileSessionRecord(w, r)
	if !ok {
		return
	}
	query, err := mobilelink.ParseTranscriptQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "历史分页参数无效", http.StatusBadRequest)
		return
	}
	s.conversationMu.Lock()
	defer s.conversationMu.Unlock()
	s.mobileCLI.mu.Lock()
	record, err := s.mobileCLI.record(s.profile, view.ID)
	s.mobileCLI.mu.Unlock()
	if err != nil {
		http.Error(w, "无法读取会话", http.StatusInternalServerError)
		return
	}
	executions := append([]claudeExecution{}, record.Executions...)
	executions = append(executions, claudeExecution{CLIID: record.CLIID, Account: record.Account, Config: record.GuestConfig})
	if query.Stream >= int64(len(executions)) {
		http.Error(w, "历史来源不存在", http.StatusNotFound)
		return
	}
	execution := executions[query.Stream]
	input := map[string]any{"cli_id": execution.CLIID, "config": execution.Config, "account": execution.Account.GuestID,
		"workspace": guestPath(record.Workspace), "offset": query.Offset, "max_bytes": query.MaxBytes, "source": query.Source,
		"required": query.Stream < int64(len(record.Executions))}
	var chunk mobilelink.TranscriptChunk
	if err := s.callHappiest(r.Context(), "read_session_history", input, &chunk); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if chunk.Error != "" {
		status := http.StatusConflict
		if chunk.Error == "transcript_unavailable" {
			status = http.StatusNotFound
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		jsonReply(w, map[string]any{"error": chunk.Error, "id": record.ID, "stream": query.Stream, "stream_count": len(executions), "cli_id": execution.CLIID})
		return
	}
	if chunk.CLIID != execution.CLIID || chunk.Offset != query.Offset || chunk.NextOffset < chunk.Offset || chunk.NextOffset > chunk.TotalBytes {
		http.Error(w, "历史来源响应无效", http.StatusBadGateway)
		return
	}
	s.account.mu.Lock()
	defer s.account.mu.Unlock()
	if !s.mobileAuthorized(r.Context()) {
		http.Error(w, errMobileLocked.Error(), http.StatusLocked)
		return
	}
	chunk.ID, chunk.Stream = record.ID, query.Stream
	chunk.StreamCount = int64(len(executions))
	if chunk.EOF && query.Stream+1 < int64(len(executions)) {
		next := query.Stream + 1
		chunk.NextStream = &next
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonReply(w, chunk)
}
