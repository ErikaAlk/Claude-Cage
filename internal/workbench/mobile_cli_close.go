package workbench

import (
	"net/http"
)

func (s *Server) mobileCloseSession(w http.ResponseWriter, r *http.Request) {
	view, _, ok := s.authorizedMobileSessionRecord(w, r)
	if !ok {
		return
	}
	var input struct{}
	if !readJSON(w, r, &input) {
		return
	}
	if r.Context().Err() != nil {
		http.Error(w, "停止请求已取消", http.StatusRequestTimeout)
		return
	}
	s.op.Lock()
	defer s.op.Unlock()
	if r.Context().Err() != nil {
		http.Error(w, "停止请求已取消", http.StatusRequestTimeout)
		return
	}
	s.conversationMu.Lock()
	s.account.mu.Lock()
	if !s.mobileAuthorized(r.Context()) {
		s.account.mu.Unlock()
		s.conversationMu.Unlock()
		http.Error(w, errMobileLocked.Error(), http.StatusLocked)
		return
	}
	s.mobileCLI.mu.Lock()
	record, err := s.mobileCLI.record(s.profile, view.ID)
	s.mobileCLI.mu.Unlock()
	if err != nil || record.Deleted {
		s.account.mu.Unlock()
		s.conversationMu.Unlock()
		http.Error(w, "会话已不可用", http.StatusNotFound)
		return
	}
	terminal := s.terminals.get(record.TerminalID)
	if terminal != nil && terminal.summary()["cli_session_id"] != record.ID {
		s.account.mu.Unlock()
		s.conversationMu.Unlock()
		http.Error(w, "终端所属会话已变化，请重新读取列表", http.StatusConflict)
		return
	}
	var stoppedRun <-chan struct{}
	s.mobileCLI.mu.Lock()
	if run := s.mobileCLI.active[view.ID]; run != nil && run.cancel != nil {
		run.cancel()
		stoppedRun = run.done
	}
	s.mobileCLI.mu.Unlock()
	s.account.mu.Unlock()
	if terminal != nil {
		s.terminals.remove(record.TerminalID)
	}
	if stoppedRun != nil {
		<-stoppedRun
	}
	s.conversationMu.Unlock()
	query := map[string]any{"cli_id": record.CLIID, "config": record.GuestConfig, "account": record.Account.GuestID}
	var current struct {
		CLIID   string `json:"cli_id"`
		Running *bool  `json:"running"`
	}
	if err := s.callHappiest(r.Context(), "check_session_process", query, &current); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if current.Running == nil || current.CLIID != record.CLIID {
		http.Error(w, "客机会话状态响应无效，尚未确认停止", http.StatusBadGateway)
		return
	}
	if *current.Running {
		http.Error(w, "原会话仍在运行，尚未确认停止；请重新读取状态后重试", http.StatusConflict)
		return
	}
	// 通过原有历史读取所有者确认下次打开需要恢复同一 Claude UUID。
	if record.Terminal {
		if _, err := s.terminalConversationView(r.Context(), record.ID); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	s.account.mu.Lock()
	defer s.account.mu.Unlock()
	if !s.mobileAuthorized(r.Context()) {
		http.Error(w, errMobileLocked.Error(), http.StatusLocked)
		return
	}
	s.mobileCLI.mu.Lock()
	defer s.mobileCLI.mu.Unlock()
	record, err = s.mobileCLI.record(s.profile, view.ID)
	if err != nil {
		http.Error(w, "无法保存会话停止状态", http.StatusInternalServerError)
		return
	}
	record.TerminalID, record.TerminalRunning, record.Running, record.Active = "", false, false, false
	record.changed()
	if err := writePrivate(mobileRecordPath(s.profile, record.ID), record); err != nil {
		http.Error(w, "无法保存会话停止状态", http.StatusInternalServerError)
		return
	}
	delete(s.mobileCLI.running, record.ID)
	delete(s.mobileCLI.desktop, record.ID)
	if s.mobileCLI.catalog != nil {
		s.mobileCLI.catalog[record.ID] = record.summary()
	}
	jsonReply(w, map[string]any{"id": record.ID, "running": false, "active": false})
}
