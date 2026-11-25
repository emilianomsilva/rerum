package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func preferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Rerum", "preferences.json"), nil
}

// LoadPreferences returns the saved UI settings, importing compatible Qt settings on first use.
func (a *App) LoadPreferences() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path, err := preferencesPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = json.Marshal(legacyPreferences(filepath.Join(filepath.Dir(path), "Desktop.conf")))
	}
	if err != nil {
		return "", err
	}
	var settings map[string]json.RawMessage
	err = json.Unmarshal(data, &settings)
	return string(data), err
}

// SavePreferences replaces the settings file atomically.
func (a *App) SavePreferences(payload string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	path, err := preferencesPath()
	if err != nil {
		return err
	}
	var settings map[string]json.RawMessage
	data := []byte(payload)
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".preferences-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func legacyPreferences(path string) map[string]json.RawMessage {
	settings := make(map[string]json.RawMessage)
	file, err := os.Open(path)
	if err != nil {
		return settings
	}
	defer file.Close()

	renamed := map[string]string{
		"window/current_page":             "app/current_page",
		"cobrancas/imovel_id":             "cobrancas/property_id",
		"movimentacoes/imovel_id":         "movimentacoes/property_id",
		"movimentacoes/empreendimento_id": "movimentacoes/project_id",
	}
	indexed := map[string]struct {
		key    string
		values []string
	}{
		"financeiro/tab_index":       {"financeiro/tab", []string{"movimentacoes", "cobrancas", "dividas"}},
		"estatisticas/tab_index":     {"estatisticas/tab", []string{"patrimonio", "irpf"}},
		"locacoes/status_index":      {"locacoes/status_filter", []string{"todas", "ativas", "encerradas"}},
		"cobrancas/filter_index":     {"cobrancas/status", []string{"todas", "a_receber", "vencidas", "a_vencer", "pagas", "canceladas"}},
		"movimentacoes/nature_index": {"movimentacoes/nature", []string{"todas", "receita", "despesa_operacional", "investimento"}},
	}

	var section string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		name, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := section + "/" + name
		var value any = raw
		switch {
		case indexed[key].key != "":
			option := indexed[key]
			index, err := strconv.Atoi(raw)
			if err != nil || index < 0 || index >= len(option.values) {
				continue
			}
			key, value = option.key, option.values[index]
		case renamed[key] != "":
			key = renamed[key]
		case strings.HasSuffix(key, "/search"),
			key == "dashboard/chart_view", key == "dashboard/chart_range",
			key == "dashboard/chart_start", key == "dashboard/chart_end",
			key == "dashboard/property_id":
			if decoded, err := strconv.Unquote(raw); err == nil {
				value = decoded
			}
		case key == "dashboard/bar_type":
			value, err = strconv.ParseBool(raw)
			if err != nil {
				continue
			}
		case key == "dashboard/visible_cards":
			value = strings.Split(raw, ", ")
		case key == "imoveis/category_index", key == "inquilinos/status_index",
			key == "empreendimentos/status_index", key == "dashboard/days_spin",
			key == "dashboard/renewal_notice_days", key == "estatisticas/year":
			value, err = strconv.Atoi(raw)
			if err != nil {
				continue
			}
		default:
			continue
		}
		settings[key], _ = json.Marshal(value)
	}
	return settings
}
