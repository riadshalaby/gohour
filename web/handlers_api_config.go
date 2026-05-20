package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/riadshalaby/gohour/config"
)

func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configResponseFromConfig(s.config.Snapshot()))
}

func (s *Server) handleAPIConfigPatch(w http.ResponseWriter, r *http.Request) {
	var body configPatchRequest
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.OnePointURL == nil {
		http.Error(w, "onepointUrl is required", http.StatusBadRequest)
		return
	}
	nextURL := strings.TrimSpace(*body.OnePointURL)
	if err := validateOnePointURL(nextURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		next.OnePoint.URL = nextURL
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, configResponseFromConfig(cfg))
}

func (s *Server) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, rulePayloadsFromRules(s.config.Snapshot().Rules))
}

func (s *Server) handleAPIRuleCreate(w http.ResponseWriter, r *http.Request) {
	var body rulePayload
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rule, err := ruleFromPayload(body, "", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		if findRuleIndex(next.Rules, rule.Name) >= 0 {
			return errRuleDuplicate
		}
		next.Rules = append(next.Rules, rule)
		return nil
	})
	if err != nil {
		if errors.Is(err, errRuleDuplicate) {
			http.Error(w, "rule already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	index := findRuleIndex(cfg.Rules, rule.Name)
	writeJSON(w, http.StatusCreated, rulePayloadFromRule(cfg.Rules[index]))
}

func (s *Server) handleAPIRulePatch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		http.Error(w, "rule name is required", http.StatusBadRequest)
		return
	}
	var body rulePayload
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rule, err := ruleFromPayload(body, name, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.config.Update(func(next *config.Config) error {
		index := findRuleIndex(next.Rules, name)
		if index < 0 {
			return errRuleNotFound
		}
		if !sameRuleName(name, rule.Name) && findRuleIndex(next.Rules, rule.Name) >= 0 {
			return errRuleDuplicate
		}
		next.Rules[index] = rule
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errRuleNotFound):
			http.Error(w, "rule not found", http.StatusNotFound)
		case errors.Is(err, errRuleDuplicate):
			http.Error(w, "rule already exists", http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	index := findRuleIndex(cfg.Rules, rule.Name)
	writeJSON(w, http.StatusOK, rulePayloadFromRule(cfg.Rules[index]))
}

func (s *Server) handleAPIRuleDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		http.Error(w, "rule name is required", http.StatusBadRequest)
		return
	}

	_, err := s.config.Update(func(next *config.Config) error {
		index := findRuleIndex(next.Rules, name)
		if index < 0 {
			return errRuleNotFound
		}
		next.Rules = append(next.Rules[:index], next.Rules[index+1:]...)
		return nil
	})
	if err != nil {
		if errors.Is(err, errRuleNotFound) {
			http.Error(w, "rule not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
