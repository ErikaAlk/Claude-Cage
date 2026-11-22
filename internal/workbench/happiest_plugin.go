package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/sylvinus/agent-vm/internal/hostbridge"
)

func (s *Server) happiestPlugin() *hostbridge.External {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hostTools == nil || slices.Contains(hostPluginsOff(s.preferences), "happiest") {
		return nil
	}
	for _, plugin := range s.hostTools.plugins {
		if plugin.Manifest.Name == "happiest" && plugin.Manifest.Kind == hostbridge.KindGuest && plugin.Running() {
			return plugin
		}
	}
	return nil
}

func (s *Server) pairedHappiest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room := s.pairedPhoneRoom(r)
		if room == nil || room.Err() != nil {
			http.Error(w, "此操作需要已配对的连接器", http.StatusForbidden)
			return
		}
		if s.happiestPlugin() == nil {
			http.Error(w, "Happiest 插件尚未运行", http.StatusServiceUnavailable)
			return
		}
		next(w, r)
	}
}

func (s *Server) callHappiest(ctx context.Context, name string, input any, out any) error {
	plugin := s.happiestPlugin()
	if plugin == nil {
		return errors.New("Happiest 插件尚未运行")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	result, err := plugin.CallFromView(ctx, name, data, 15*time.Second)
	if err != nil || result == nil || result.IsError || result.StructuredContent == nil {
		return errors.New("Happiest 插件未能完成会话请求")
	}
	if s.happiestPlugin() != plugin || ctx.Err() != nil {
		return errors.New("Happiest 插件调用已取消")
	}
	data, err = json.Marshal(result.StructuredContent)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
